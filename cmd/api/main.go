package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/config"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/logger"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/telemetry"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	redisStorage "github.com/IgorGrieder/encurtador-url/internal/storage/redis"
	"github.com/IgorGrieder/encurtador-url/internal/storage/mongo"
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

	logger.Info("Starting application",
		zap.String("name", cfg.App.Name),
		zap.String("version", cfg.App.Version),
		zap.String("env", cfg.App.Env),
	)

	var shutdownTracer func(context.Context) error
	if cfg.OTel.Enabled {
		var err error
		shutdownTracer, err = telemetry.InitTracer(cfg.OTel.Endpoint, cfg.App.Name, cfg.App.Version)
		if err != nil {
			logger.Warn("Failed to initialize tracer, continuing without tracing", zap.Error(err))
		} else {
			logger.Info("OpenTelemetry tracer initialized", zap.String("endpoint", cfg.OTel.Endpoint))
		}
	}

	mongoConn, err := db.ConnectMongo(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() { _ = mongoConn.Disconnect() }()

	linkRepo, err := mongo.NewLinksRepository(mongoConn)
	if err != nil {
		logger.Fatal("Failed to initialize links repository", zap.Error(err))
	}
	statsRepo, err := mongo.NewClickStatsRepository(mongoConn)
	if err != nil {
		logger.Fatal("Failed to initialize click stats repository", zap.Error(err))
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
	router := httpTransport.NewRouter(cfg, linkSvc, createLimiter)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if shutdownTracer != nil {
			_ = shutdownTracer(shutdownCtx)
		}

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("Server shutdown error", zap.Error(err))
		}
	}()

	logger.Info("Server starting",
		zap.String("port", cfg.Server.Port),
		zap.String("env", cfg.App.Env),
		zap.String("address", fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port)),
	)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		logger.Fatal("Server error", zap.Error(err))
	}

	logger.Info("Server stopped gracefully")
}
