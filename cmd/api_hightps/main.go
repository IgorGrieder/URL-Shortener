package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/config"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/logger"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/telemetry"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	mongoStorage "github.com/IgorGrieder/encurtador-url/internal/storage/mongo"
	redisStorage "github.com/IgorGrieder/encurtador-url/internal/storage/redis"
	httpTransport "github.com/IgorGrieder/encurtador-url/internal/transport/http"
	"github.com/IgorGrieder/encurtador-url/internal/transport/http/middleware"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if err := logger.Init(cfg.App.Env); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("Starting application (high TPS)",
		zap.String("name", cfg.App.Name),
		zap.String("version", cfg.App.Version),
		zap.String("env", cfg.App.Env),
	)

	var shutdownTracer func(context.Context) error
	if cfg.OTel.Enabled {
		shutdownTracer, err = telemetry.InitTracer(cfg.OTel.Endpoint, cfg.App.Name, cfg.App.Version)
		if err != nil {
			logger.Warn("Failed to initialize tracer, continuing without tracing", zap.Error(err))
			shutdownTracer = nil
		} else {
			logger.Info("OpenTelemetry tracer initialized", zap.String("endpoint", cfg.OTel.Endpoint))
		}
	}

	mongoConn, err := db.ConnectMongo(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() { _ = mongoConn.Disconnect() }()

	mongoLinkRepo, err := mongoStorage.NewLinksRepository(mongoConn)
	if err != nil {
		logger.Fatal("Failed to initialize links repository", zap.Error(err))
	}

	linkRepo := links.LinkRepository(mongoLinkRepo)

	mongoStatsRepo, err := mongoStorage.NewClickStatsRepository(mongoConn)
	if err != nil {
		logger.Fatal("Failed to initialize click stats repository", zap.Error(err))
	}

	var statsShutdown func(context.Context) error
	var bufferedStats *mongoStorage.BufferedClickStatsRepository
	statsRepo := links.StatsRepository(mongoStatsRepo)
	if getEnvBool("CLICK_BUFFER_ENABLED", true) {
		buffered := mongoStorage.NewBufferedClickStatsRepository(mongoStatsRepo, mongoStorage.BufferedClickStatsOptions{
			QueueSize:      getEnvInt("CLICK_BUFFER_QUEUE_SIZE", 1_000_000),
			FlushInterval:  getEnvDuration("CLICK_BUFFER_FLUSH_INTERVAL", 250*time.Millisecond),
			MaxBatchEvents: getEnvInt("CLICK_BUFFER_MAX_BATCH_EVENTS", 50_000),
			FlushTimeout:   getEnvDuration("CLICK_BUFFER_FLUSH_TIMEOUT", 2*time.Second),
		})
		statsRepo = buffered
		statsShutdown = buffered.Shutdown
		bufferedStats = buffered
	}

	linkSvc := links.NewService(linkRepo, statsRepo, links.NewCryptoSlugger(), cfg.Shortener.SlugLength)

	redisClient, err := redisStorage.New(redisStorage.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	defer func() { _ = redisClient.Close() }()

	redisLimiterStore := redisStorage.NewFixedWindowLimiter(redisClient, "rl:create", time.Minute)
	createLimiter := middleware.NewRedisFixedWindowLimiter(redisLimiterStore, cfg.Security.CreateRate.RequestsPerMinute)

	routerOpts := httpTransport.DefaultRouterOptions()
	routerOpts.EnableCORS = getEnvBool("HTTP_ENABLE_CORS", true)
	routerOpts.EnableLogging = getEnvBool("HTTP_ENABLE_LOGGING", false)
	routerOpts.EnableMetrics = getEnvBool("HTTP_ENABLE_METRICS", false)
	routerOpts.EnableTracing = getEnvBool("HTTP_ENABLE_TRACING", cfg.OTel.Enabled)
	routerOpts.LinksHandlerOptions = httpTransport.LinksHandlerOptions{
		AsyncClick:   getEnvBool("REDIRECT_ASYNC_CLICK", false),
		ClickTimeout: getEnvDuration("REDIRECT_CLICK_TIMEOUT", 2*time.Second),
		FastRedirect: getEnvBool("REDIRECT_FAST", true),
	}

	router := httpTransport.NewRouterWithOptions(cfg, linkSvc, createLimiter, routerOpts)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrCh := make(chan error, 1)
	go func() {
		serverErrCh <- server.ListenAndServe()
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Server starting",
		zap.String("port", cfg.Server.Port),
		zap.String("env", cfg.App.Env),
		zap.String("address", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)),
	)

	select {
	case err := <-serverErrCh:
		if err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server error", zap.Error(err))
		}
	case sig := <-sigChan:
		logger.Info("Shutting down server...", zap.String("signal", sig.String()))

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server shutdown error", zap.Error(err))
		}

		if statsShutdown != nil {
			if err := statsShutdown(shutdownCtx); err != nil {
				logger.Warn("Click buffer shutdown error", zap.Error(err))
			}
			if bufferedStats != nil {
				logger.Info("Click buffer drained", zap.Int64("dropped", bufferedStats.Dropped()))
			}
		}

		if shutdownTracer != nil {
			_ = shutdownTracer(shutdownCtx)
		}
	}

	logger.Info("Server stopped gracefully")
}

func getEnvInt(key string, defaultValue int) int {
	if raw := os.Getenv(key); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if raw := os.Getenv(key); raw != "" {
		if v, err := strconv.ParseBool(raw); err == nil {
			return v
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if raw := os.Getenv(key); raw != "" {
		if v, err := time.ParseDuration(raw); err == nil {
			return v
		}
	}
	return defaultValue
}
