package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/storage/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClickEventProcessor struct {
	pool *pgxpool.Pool
}

func NewClickEventProcessor(p *db.Postgres) (*ClickEventProcessor, error) {
	if p == nil || p.Pool == nil {
		return nil, errors.New("postgres pool is nil")
	}
	return &ClickEventProcessor{pool: p.Pool}, nil
}

func (p *ClickEventProcessor) Process(
	ctx context.Context,
	eventID string,
	slug string,
	occurredAt time.Time,
) (alreadyProcessed bool, countersApplied bool, err error) {
	eventID = strings.TrimSpace(eventID)
	slug = strings.TrimSpace(slug)
	if eventID == "" {
		return false, false, errors.New("eventID must not be empty")
	}
	if slug == "" {
		return false, false, errors.New("slug must not be empty")
	}

	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, false, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	queries := sqlc.New(tx)
	insertedRows, err := queries.InsertProcessedEventOnce(ctx, sqlc.InsertProcessedEventOnceParams{
		EventID:     eventID,
		ProcessedAt: processorToTimestamptz(time.Now().UTC()),
	})
	if err != nil {
		return false, false, err
	}

	if insertedRows == 0 {
		if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			return false, false, err
		}
		tx = nil
		return true, false, nil
	}

	_, err = queries.GetActiveLinkBySlugAndIncClick(ctx, sqlc.GetActiveLinkBySlugAndIncClickParams{
		Slug:      slug,
		ExpiresAt: processorToTimestamptz(occurredAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if err := tx.Commit(ctx); err != nil {
				return false, false, err
			}
			tx = nil
			return false, false, nil
		}
		return false, false, err
	}

	if err := queries.IncDailyClick(ctx, sqlc.IncDailyClickParams{
		Slug: slug,
		Day:  processorToDate(occurredAt),
	}); err != nil {
		return false, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, false, err
	}
	tx = nil
	return false, true, nil
}

func processorToTimestamptz(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  v.UTC(),
		Valid: true,
	}
}

func processorToDate(v time.Time) pgtype.Date {
	y, m, d := v.UTC().Date()
	return pgtype.Date{
		Time:  time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}
