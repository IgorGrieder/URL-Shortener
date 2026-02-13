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
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/logger"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/telemetry"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	httpTransport "github.com/IgorGrieder/encurtador-url/internal/transport/http"
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
	shutdownTracer, err = telemetry.InitTracer(cfg.OTel.Endpoint, cfg.App.Name, cfg.App.Version)
	if err != nil {
		logger.Warn("Failed to initialize tracer, continuing without tracing", zap.Error(err))
		shutdownTracer = nil
	} else {
		logger.Info("OpenTelemetry tracer initialized", zap.String("endpoint", cfg.OTel.Endpoint))
	}

	linkRepo, statsRepo, outboxRepo, closeStorage, err := initStorage(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize storage", zap.Error(err))
	}
	defer closeStorage()

	linkSvc := links.NewService(linkRepo, statsRepo, outboxRepo, links.NewCryptoSlugger(), cfg.Shortener.SlugLength)

	router := httpTransport.NewRouter(cfg, linkSvc)

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

		if shutdownTracer != nil {
			_ = shutdownTracer(shutdownCtx)
		}
	}

	logger.Info("Server stopped gracefully")
}
