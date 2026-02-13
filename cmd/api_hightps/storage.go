package main

import (
	"context"
	"fmt"

	"github.com/IgorGrieder/encurtador-url/internal/config"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/logger"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	postgresStorage "github.com/IgorGrieder/encurtador-url/internal/storage/postgres"
	"go.uber.org/zap"
)

func initStorage(cfg *config.Config) (
	links.LinkRepository,
	links.StatsRepository,
	links.ClickOutboxRepository,
	func(),
	error,
) {
	pgConn, err := db.ConnectPostgres(context.Background(), cfg.Postgres.DSN())
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("connect postgres: %w", err)
	}

	linkRepo, err := postgresStorage.NewLinksRepository(pgConn)
	if err != nil {
		pgConn.Close()
		return nil, nil, nil, nil, fmt.Errorf("init postgres links repository: %w", err)
	}
	statsRepo, err := postgresStorage.NewClickStatsRepository(pgConn)
	if err != nil {
		pgConn.Close()
		return nil, nil, nil, nil, fmt.Errorf("init postgres stats repository: %w", err)
	}
	outboxRepo, err := postgresStorage.NewClickOutboxRepository(pgConn)
	if err != nil {
		pgConn.Close()
		return nil, nil, nil, nil, fmt.Errorf("init postgres outbox repository: %w", err)
	}

	logger.Info("Storage backend selected", zap.String("backend", "postgres"))
	return linkRepo, statsRepo, outboxRepo, pgConn.Close, nil
}
