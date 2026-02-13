package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"github.com/IgorGrieder/encurtador-url/internal/storage/postgres/sqlc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type LinksRepository struct {
	queries *sqlc.Queries
}

func NewLinksRepository(p *db.Postgres) (*LinksRepository, error) {
	if p == nil || p.Pool == nil {
		return nil, errors.New("postgres pool is nil")
	}
	return &LinksRepository{queries: sqlc.New(p.Pool)}, nil
}

func (r *LinksRepository) Insert(ctx context.Context, link *links.Link) error {
	if link == nil {
		return errors.New("link is nil")
	}

	_, err := r.queries.CreateLink(ctx, sqlc.CreateLinkParams{
		Slug:      link.Slug,
		Url:       link.URL,
		Notes:     toNullableText(link.Notes),
		ApiKey:    toNullableText(link.APIKey),
		CreatedAt: toTimestamptz(link.CreatedAt),
		ExpiresAt: toNullableTimestamptz(link.ExpiresAt),
		Clicks:    link.Clicks,
	})
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return links.ErrSlugTaken
	}
	return err
}

func (r *LinksRepository) FindBySlug(ctx context.Context, slug string) (*links.Link, error) {
	row, err := r.queries.GetLinkBySlug(ctx, slug)
	if err == nil {
		return mapLinkRow(row), nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, links.ErrNotFound
	}
	return nil, err
}

func (r *LinksRepository) FindActiveBySlug(ctx context.Context, slug string, at time.Time) (*links.Link, error) {
	row, err := r.queries.GetActiveLinkBySlug(ctx, sqlc.GetActiveLinkBySlugParams{
		Slug:      slug,
		ExpiresAt: toTimestamptz(at),
	})
	if err == nil {
		return mapLinkRow(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	existing, findErr := r.FindBySlug(ctx, slug)
	if findErr == nil && existing != nil {
		return nil, links.ErrExpired
	}
	if findErr != nil {
		return nil, findErr
	}
	return nil, links.ErrNotFound
}

func (r *LinksRepository) FindActiveBySlugAndIncClick(ctx context.Context, slug string, at time.Time) (*links.Link, error) {
	row, err := r.queries.GetActiveLinkBySlugAndIncClick(ctx, sqlc.GetActiveLinkBySlugAndIncClickParams{
		Slug:      slug,
		ExpiresAt: toTimestamptz(at),
	})
	if err == nil {
		return mapLinkRow(row), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	existing, findErr := r.FindBySlug(ctx, slug)
	if findErr == nil && existing != nil {
		return nil, links.ErrExpired
	}
	if findErr != nil {
		return nil, findErr
	}
	return nil, links.ErrNotFound
}

func (r *LinksRepository) DeleteBySlug(ctx context.Context, slug string) (bool, error) {
	rows, err := r.queries.DeleteLinkBySlug(ctx, slug)
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func mapLinkRow(row sqlc.Link) *links.Link {
	out := &links.Link{
		Slug:      row.Slug,
		URL:       row.Url,
		Notes:     nullableTextValue(row.Notes),
		APIKey:    nullableTextValue(row.ApiKey),
		CreatedAt: row.CreatedAt.Time.UTC(),
		Clicks:    row.Clicks,
	}

	if row.ExpiresAt.Valid {
		t := row.ExpiresAt.Time.UTC()
		out.ExpiresAt = &t
	}

	return out
}

func toNullableText(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{
		String: v,
		Valid:  true,
	}
}

func nullableTextValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func toTimestamptz(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  v.UTC(),
		Valid: true,
	}
}

func toNullableTimestamptz(v *time.Time) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return toTimestamptz(*v)
}
