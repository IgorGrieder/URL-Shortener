package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/config"
	"github.com/IgorGrieder/encurtador-url/internal/events"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/logger"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/telemetry"
	postgresStorage "github.com/IgorGrieder/encurtador-url/internal/storage/postgres"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type consumerConfig struct {
	appEnv       string
	appName      string
	appVersion   string
	otelEndpoint string
	postgresDSN  string

	kafkaBrokers []string
	kafkaTopic   string
	kafkaGroupID string
	workerID     string

	fetchMaxWait   time.Duration
	operationTTL   time.Duration
	consumeBackoff time.Duration
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.appEnv); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	shutdownTracer, err := telemetry.InitTracer(
		cfg.otelEndpoint,
		fmt.Sprintf("%s-click-consumer", cfg.appName),
		cfg.appVersion,
	)
	if err != nil {
		logger.Warn("failed to initialize tracer, continuing without tracing", zap.Error(err))
		shutdownTracer = nil
	} else {
		logger.Info("OpenTelemetry tracer initialized",
			zap.String("endpoint", cfg.otelEndpoint),
			zap.String("service", fmt.Sprintf("%s-click-consumer", cfg.appName)),
		)
	}
	defer func() {
		if shutdownTracer == nil {
			return
		}
		if err := shutdownTracer(context.Background()); err != nil {
			logger.Warn("failed to shutdown tracer", zap.Error(err))
		}
	}()

	pgConn, err := db.ConnectPostgres(context.Background(), cfg.postgresDSN)
	if err != nil {
		logger.Fatal("failed to connect to PostgreSQL", zap.Error(err))
	}
	defer pgConn.Close()

	clickProcessor, err := postgresStorage.NewClickEventProcessor(pgConn)
	if err != nil {
		logger.Fatal("failed to initialize click event processor", zap.Error(err))
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.kafkaBrokers,
		Topic:       cfg.kafkaTopic,
		GroupID:     cfg.kafkaGroupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		MaxWait:     cfg.fetchMaxWait,
		StartOffset: kafka.FirstOffset,
	})
	defer func() {
		if err := reader.Close(); err != nil {
			logger.Warn("failed to close kafka reader", zap.Error(err))
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("click consumer started",
		zap.Strings("kafka_brokers", cfg.kafkaBrokers),
		zap.String("kafka_topic", cfg.kafkaTopic),
		zap.String("kafka_group", cfg.kafkaGroupID),
		zap.String("worker_id", cfg.workerID),
	)

	tracer := otel.Tracer("click-consumer")
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info("click consumer stopping")
				return
			}
			logger.Error("failed to fetch kafka message", zap.Error(err))
			time.Sleep(cfg.consumeBackoff)
			continue
		}

		consumeCtx := contextFromKafkaHeaders(ctx, msg.Headers)
		consumeCtx, span := tracer.Start(
			consumeCtx,
			"kafka.consume.click_recorded",
			trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination.name", msg.Topic),
				attribute.String("messaging.operation", "process"),
				attribute.Int("messaging.kafka.partition", msg.Partition),
				attribute.Int64("messaging.kafka.offset", msg.Offset),
			),
		)

		if err := processMessage(
			consumeCtx,
			msg,
			clickProcessor,
			cfg.operationTTL,
		); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "process click event failed")
			logger.Error("failed to process click event",
				zap.Error(err),
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
			)
			span.End()
			time.Sleep(cfg.consumeBackoff)
			continue
		}

		if err := reader.CommitMessages(consumeCtx, msg); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "commit kafka offset failed")
			logger.Error("failed to commit kafka offset",
				zap.Error(err),
				zap.String("topic", msg.Topic),
				zap.Int("partition", msg.Partition),
				zap.Int64("offset", msg.Offset),
			)
			span.End()
			time.Sleep(cfg.consumeBackoff)
			continue
		}

		span.End()
	}
}

func processMessage(
	ctx context.Context,
	msg kafka.Message,
	clickProcessor *postgresStorage.ClickEventProcessor,
	operationTTL time.Duration,
) error {
	var event events.ClickRecorded
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		logger.Warn("invalid click event payload, skipping",
			zap.Error(err),
			zap.ByteString("payload", msg.Value),
		)
		return nil
	}
	if strings.TrimSpace(event.Slug) == "" {
		logger.Warn("click event missing slug, skipping", zap.String("event_id", event.EventID))
		return nil
	}
	if strings.TrimSpace(event.EventID) == "" {
		logger.Warn("click event missing eventId, skipping", zap.ByteString("payload", msg.Value))
		return nil
	}

	occurredAt := msg.Time.UTC()
	if strings.TrimSpace(event.OccurredAt) != "" {
		parsed, err := time.Parse(time.RFC3339Nano, event.OccurredAt)
		if err != nil {
			logger.Warn("invalid event occurredAt, using kafka timestamp",
				zap.Error(err),
				zap.String("event_id", event.EventID),
			)
		} else {
			occurredAt = parsed.UTC()
		}
	}

	opCtx, cancel := context.WithTimeout(ctx, operationTTL)
	defer cancel()

	alreadyProcessed, countersApplied, err := clickProcessor.Process(opCtx, event.EventID, event.Slug, occurredAt)
	if err != nil {
		return err
	}
	if alreadyProcessed {
		return nil
	}
	if !countersApplied {
		logger.Info("click event skipped for missing or expired link",
			zap.String("event_id", event.EventID),
			zap.String("slug", event.Slug),
		)
	}

	return nil
}

func loadConfig() (cfg consumerConfig, _ error) {
	cfg = consumerConfig{
		appEnv:         config.GetEnv("APP_ENV", "production"),
		appName:        config.GetEnv("APP_NAME", "encurtador-url"),
		appVersion:     config.GetEnv("APP_VERSION", "0.1.0"),
		otelEndpoint:   config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://jaeger:4318"),
		postgresDSN:    config.GetEnv("DB_DSN", config.DefaultPostgresDSN()),
		kafkaBrokers:   config.SplitCSV(config.GetEnv("KAFKA_BROKERS", "kafka:9092")),
		kafkaTopic:     config.GetEnv("KAFKA_CLICK_TOPIC", "clicks.recorded"),
		kafkaGroupID:   config.GetEnv("KAFKA_CLICK_GROUP_ID", "click-analytics"),
		workerID:       config.GetEnv("KAFKA_CONSUMER_WORKER_ID", config.DefaultWorkerID("click-consumer")),
		fetchMaxWait:   config.GetEnvDuration("KAFKA_CONSUMER_MAX_WAIT", 500*time.Millisecond),
		operationTTL:   config.GetEnvDuration("KAFKA_CONSUMER_OPERATION_TIMEOUT", 5*time.Second),
		consumeBackoff: config.GetEnvDuration("KAFKA_CONSUMER_BACKOFF", 500*time.Millisecond),
	}

	if strings.TrimSpace(cfg.postgresDSN) == "" {
		return consumerConfig{}, fmt.Errorf("DB_DSN must not be empty")
	}
	if len(cfg.kafkaBrokers) == 0 {
		return consumerConfig{}, fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}
	if strings.TrimSpace(cfg.kafkaTopic) == "" {
		return consumerConfig{}, fmt.Errorf("KAFKA_CLICK_TOPIC must not be empty")
	}
	if strings.TrimSpace(cfg.kafkaGroupID) == "" {
		return consumerConfig{}, fmt.Errorf("KAFKA_CLICK_GROUP_ID must not be empty")
	}
	if cfg.operationTTL <= 0 {
		return consumerConfig{}, fmt.Errorf("KAFKA_CONSUMER_OPERATION_TIMEOUT must be > 0")
	}
	if strings.TrimSpace(cfg.workerID) == "" {
		return consumerConfig{}, fmt.Errorf("KAFKA_CONSUMER_WORKER_ID must not be empty")
	}

	return cfg, nil
}

func contextFromKafkaHeaders(parent context.Context, headers []kafka.Header) context.Context {
	carrier := propagation.MapCarrier{}
	for _, header := range headers {
		key := strings.ToLower(strings.TrimSpace(header.Key))
		if key == "" {
			continue
		}
		carrier.Set(key, string(header.Value))
	}
	return otel.GetTextMapPropagator().Extract(parent, carrier)
}
