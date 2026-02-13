package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"github.com/IgorGrieder/encurtador-url/internal/storage/postgres/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

type ClickStatsRepository struct {
	queries *sqlc.Queries
}

func NewClickStatsRepository(p *db.Postgres) (*ClickStatsRepository, error) {
	if p == nil || p.Pool == nil {
		return nil, errors.New("postgres pool is nil")
	}
	return &ClickStatsRepository{queries: sqlc.New(p.Pool)}, nil
}

func (r *ClickStatsRepository) IncDaily(ctx context.Context, slug string, at time.Time) error {
	return r.queries.IncDailyClick(ctx, sqlc.IncDailyClickParams{
		Slug: slug,
		Day:  toDate(at),
	})
}

func (r *ClickStatsRepository) GetDaily(ctx context.Context, slug string, from, to time.Time) ([]links.DailyCount, error) {
	rows, err := r.queries.GetDailyStatsByRange(ctx, sqlc.GetDailyStatsByRangeParams{
		Slug:  slug,
		Day:   toDate(from),
		Day_2: toDate(to),
	})
	if err != nil {
		return nil, err
	}

	out := make([]links.DailyCount, 0, len(rows))
	for _, row := range rows {
		day := ""
		if row.Day.Valid {
			day = row.Day.Time.Format(time.DateOnly)
		}
		out = append(out, links.DailyCount{
			Date:  day,
			Count: row.Count,
		})
	}

	return out, nil
}

func (r *ClickStatsRepository) DeleteBySlug(ctx context.Context, slug string) error {
	return r.queries.DeleteDailyStatsBySlug(ctx, slug)
}

func toDate(v time.Time) pgtype.Date {
	y, m, d := v.UTC().Date()
	return pgtype.Date{
		Time:  time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}
