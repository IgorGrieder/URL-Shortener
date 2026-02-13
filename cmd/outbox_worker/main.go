package main

import (
	"context"
	"encoding/json"
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

type workerConfig struct {
	appEnv       string
	appName      string
	appVersion   string
	otelEndpoint string
	postgresDSN  string

	kafkaBrokers []string
	kafkaTopic   string
	workerID     string

	pollInterval time.Duration
	batchSize    int
	writeTimeout time.Duration
	retryBase    time.Duration
	retryMax     time.Duration
	idleWait     time.Duration
	claimLease   time.Duration
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
		fmt.Sprintf("%s-outbox-worker", cfg.appName),
		cfg.appVersion,
	)
	if err != nil {
		logger.Warn("failed to initialize tracer, continuing without tracing", zap.Error(err))
		shutdownTracer = nil
	} else {
		logger.Info("OpenTelemetry tracer initialized",
			zap.String("endpoint", cfg.otelEndpoint),
			zap.String("service", fmt.Sprintf("%s-outbox-worker", cfg.appName)),
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

	outboxRepo, err := postgresStorage.NewClickOutboxRepository(pgConn)
	if err != nil {
		logger.Fatal("failed to initialize outbox repository", zap.Error(err))
	}

	writer := kafka.Writer{
		Addr:                   kafka.TCP(cfg.kafkaBrokers...),
		Topic:                  cfg.kafkaTopic,
		Balancer:               &kafka.LeastBytes{},
		BatchTimeout:           10 * time.Millisecond,
		RequiredAcks:           kafka.RequireOne,
		AllowAutoTopicCreation: true,
	}
	defer func() {
		if err := writer.Close(); err != nil {
			logger.Warn("failed to close kafka writer", zap.Error(err))
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("outbox worker started",
		zap.Strings("kafka_brokers", cfg.kafkaBrokers),
		zap.String("kafka_topic", cfg.kafkaTopic),
		zap.String("worker_id", cfg.workerID),
		zap.Int("batch_size", cfg.batchSize),
		zap.Duration("poll_interval", cfg.pollInterval),
		zap.Duration("claim_lease", cfg.claimLease),
	)

	ticker := time.NewTicker(cfg.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("outbox worker stopping")
			return
		default:
		}

		processed, err := processBatch(ctx, outboxRepo, &writer, cfg)
		if err != nil {
			logger.Error("failed to process outbox batch", zap.Error(err))
		}

		if processed == 0 {
			select {
			case <-ctx.Done():
				logger.Info("outbox worker stopping")
				return
			case <-ticker.C:
			}
			continue
		}

		if cfg.idleWait > 0 {
			select {
			case <-ctx.Done():
				logger.Info("outbox worker stopping")
				return
			case <-time.After(cfg.idleWait):
			}
		}
	}
}

func processBatch(
	ctx context.Context,
	repo *postgresStorage.ClickOutboxRepository,
	writer *kafka.Writer,
	cfg workerConfig,
) (int, error) {
	eventsBatch, err := repo.ClaimPending(ctx, time.Now().UTC(), int64(cfg.batchSize), cfg.workerID, cfg.claimLease)
	if err != nil {
		return 0, err
	}
	if len(eventsBatch) == 0 {
		return 0, nil
	}

	processed := 0
	tracer := otel.Tracer("outbox-worker")
	for _, ev := range eventsBatch {
		msgPayload := events.ClickRecorded{
			EventID:    ev.ID,
			Slug:       ev.Slug,
			OccurredAt: ev.OccurredAt.UTC().Format(time.RFC3339Nano),
		}
		value, err := json.Marshal(msgPayload)
		if err != nil {
			logger.Error("failed to marshal outbox event", zap.Error(err), zap.String("event_id", ev.ID))
			delay := backoffDelay(cfg.retryBase, cfg.retryMax, ev.Attempts+1)
			_ = repo.MarkRetry(ctx, ev.ID, cfg.workerID, truncateErr(err), time.Now().UTC().Add(delay))
			continue
		}

		carrier := outboxEventCarrier(ev)
		parentCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)
		producerCtx, span := tracer.Start(
			parentCtx,
			"kafka.publish.click_recorded",
			trace.WithSpanKind(trace.SpanKindProducer),
			trace.WithAttributes(
				attribute.String("messaging.system", "kafka"),
				attribute.String("messaging.destination.name", cfg.kafkaTopic),
				attribute.String("messaging.operation", "publish"),
				attribute.String("messaging.message.id", ev.ID),
				attribute.String("messaging.kafka.message_key", ev.Slug),
			),
		)
		otel.GetTextMapPropagator().Inject(producerCtx, carrier)

		writeCtx, cancel := context.WithTimeout(producerCtx, cfg.writeTimeout)
		err = writer.WriteMessages(writeCtx, kafka.Message{
			Key:     []byte(ev.Slug),
			Value:   value,
			Time:    ev.OccurredAt.UTC(),
			Headers: carrierToKafkaHeaders(carrier),
		})
		cancel()
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "kafka publish failed")
			delay := backoffDelay(cfg.retryBase, cfg.retryMax, ev.Attempts+1)
			if markErr := repo.MarkRetry(ctx, ev.ID, cfg.workerID, truncateErr(err), time.Now().UTC().Add(delay)); markErr != nil {
				span.RecordError(markErr)
				logger.Error("failed to mark outbox retry", zap.Error(markErr), zap.String("event_id", ev.ID))
			}
			logger.Warn("failed to publish outbox event",
				zap.Error(err),
				zap.String("event_id", ev.ID),
				zap.String("slug", ev.Slug),
				zap.Duration("retry_in", delay),
			)
			span.End()
			continue
		}

		if err := repo.MarkSent(ctx, ev.ID, cfg.workerID); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "mark sent failed")
			logger.Error("failed to mark outbox event as sent", zap.Error(err), zap.String("event_id", ev.ID))
			span.End()
			continue
		}

		span.End()
		processed++
	}

	return processed, nil
}

func loadConfig() (cfg workerConfig, _ error) {
	cfg = workerConfig{
		appEnv:       config.GetEnv("APP_ENV", "production"),
		appName:      config.GetEnv("APP_NAME", "encurtador-url"),
		appVersion:   config.GetEnv("APP_VERSION", "0.1.0"),
		otelEndpoint: config.GetEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://jaeger:4318"),
		postgresDSN:  config.GetEnv("DB_DSN", config.DefaultPostgresDSN()),
		kafkaBrokers: config.SplitCSV(config.GetEnv("KAFKA_BROKERS", "kafka:9092")),
		kafkaTopic:   config.GetEnv("KAFKA_CLICK_TOPIC", "clicks.recorded"),
		workerID:     config.GetEnv("OUTBOX_WORKER_ID", config.DefaultWorkerID("outbox-worker")),
		pollInterval: config.GetEnvDuration("OUTBOX_POLL_INTERVAL", 250*time.Millisecond),
		batchSize:    config.GetEnvInt("OUTBOX_BATCH_SIZE", 200),
		writeTimeout: config.GetEnvDuration("OUTBOX_WRITE_TIMEOUT", 5*time.Second),
		retryBase:    config.GetEnvDuration("OUTBOX_RETRY_BASE_DELAY", 1*time.Second),
		retryMax:     config.GetEnvDuration("OUTBOX_RETRY_MAX_DELAY", 30*time.Second),
		idleWait:     config.GetEnvDuration("OUTBOX_IDLE_WAIT", 50*time.Millisecond),
		claimLease:   config.GetEnvDuration("OUTBOX_CLAIM_LEASE", 30*time.Second),
	}

	if strings.TrimSpace(cfg.postgresDSN) == "" {
		return workerConfig{}, fmt.Errorf("DB_DSN must not be empty")
	}
	if len(cfg.kafkaBrokers) == 0 {
		return workerConfig{}, fmt.Errorf("KAFKA_BROKERS must contain at least one broker")
	}
	if cfg.batchSize <= 0 {
		return workerConfig{}, fmt.Errorf("OUTBOX_BATCH_SIZE must be > 0")
	}
	if cfg.pollInterval <= 0 {
		return workerConfig{}, fmt.Errorf("OUTBOX_POLL_INTERVAL must be > 0")
	}
	if cfg.writeTimeout <= 0 {
		return workerConfig{}, fmt.Errorf("OUTBOX_WRITE_TIMEOUT must be > 0")
	}
	if cfg.retryBase <= 0 {
		return workerConfig{}, fmt.Errorf("OUTBOX_RETRY_BASE_DELAY must be > 0")
	}
	if cfg.retryMax < cfg.retryBase {
		return workerConfig{}, fmt.Errorf("OUTBOX_RETRY_MAX_DELAY must be >= OUTBOX_RETRY_BASE_DELAY")
	}
	if strings.TrimSpace(cfg.workerID) == "" {
		return workerConfig{}, fmt.Errorf("OUTBOX_WORKER_ID must not be empty")
	}
	if cfg.claimLease <= 0 {
		return workerConfig{}, fmt.Errorf("OUTBOX_CLAIM_LEASE must be > 0")
	}

	return cfg, nil
}

func backoffDelay(base, max time.Duration, attempt int) time.Duration {
	delay := base
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay >= max {
			return max
		}
	}
	if delay > max {
		return max
	}
	return delay
}

func truncateErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 1000 {
		return msg[:1000]
	}
	return msg
}

func outboxEventCarrier(ev postgresStorage.OutboxClickEvent) propagation.MapCarrier {
	carrier := propagation.MapCarrier{}
	if strings.TrimSpace(ev.TraceParent) != "" {
		carrier.Set("traceparent", strings.TrimSpace(ev.TraceParent))
	}
	if strings.TrimSpace(ev.TraceState) != "" {
		carrier.Set("tracestate", strings.TrimSpace(ev.TraceState))
	}
	if strings.TrimSpace(ev.Baggage) != "" {
		carrier.Set("baggage", strings.TrimSpace(ev.Baggage))
	}
	return carrier
}

func carrierToKafkaHeaders(carrier propagation.MapCarrier) []kafka.Header {
	headers := make([]kafka.Header, 0, len(carrier))
	for key, value := range carrier {
		if strings.TrimSpace(value) == "" {
			continue
		}
		headers = append(headers, kafka.Header{
			Key:   key,
			Value: []byte(value),
		})
	}
	return headers
}
