package postgres

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/db"
	"github.com/IgorGrieder/encurtador-url/internal/storage/postgres/sqlc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	outboxStatusPending = "pending"
)

var ErrOutboxEventNotOwned = errors.New("outbox event not owned by worker")

type ClickOutboxRepository struct {
	queries *sqlc.Queries
}

type OutboxClickEvent struct {
	ID          string
	Slug        string
	OccurredAt  time.Time
	TraceParent string
	TraceState  string
	Baggage     string
	Attempts    int
}

func NewClickOutboxRepository(p *db.Postgres) (*ClickOutboxRepository, error) {
	if p == nil || p.Pool == nil {
		return nil, errors.New("postgres pool is nil")
	}
	return &ClickOutboxRepository{queries: sqlc.New(p.Pool)}, nil
}

func (r *ClickOutboxRepository) EnqueueClick(ctx context.Context, slug string, occurredAt time.Time) error {
	now := time.Now().UTC()
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	_, err := r.queries.EnqueueClickOutbox(ctx, sqlc.EnqueueClickOutboxParams{
		EventType:     "click.recorded",
		Slug:          slug,
		OccurredAt:    toOutboxTimestamptz(occurredAt),
		Traceparent:   toOutboxNullableText(carrier.Get("traceparent")),
		Tracestate:    toOutboxNullableText(carrier.Get("tracestate")),
		Baggage:       toOutboxNullableText(carrier.Get("baggage")),
		Status:        outboxStatusPending,
		NextAttemptAt: toOutboxTimestamptz(now),
		CreatedAt:     toOutboxTimestamptz(now),
	})
	return err
}

func (r *ClickOutboxRepository) ClaimPending(
	ctx context.Context,
	now time.Time,
	limit int64,
	workerID string,
	lease time.Duration,
) ([]OutboxClickEvent, error) {
	if limit <= 0 {
		limit = 1
	}
	if lease <= 0 {
		lease = 30 * time.Second
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, errors.New("workerID must not be empty")
	}

	now = now.UTC()
	events := make([]OutboxClickEvent, 0, limit)
	for int64(len(events)) < limit {
		row, err := r.queries.ClaimNextOutboxEvent(ctx, sqlc.ClaimNextOutboxEventParams{
			UpdatedAt:           toOutboxTimestamptz(now),
			ProcessingOwner:     toOutboxNullableText(workerID),
			ProcessingExpiresAt: toOutboxTimestamptz(now.Add(lease)),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			break
		}
		if err != nil {
			return nil, err
		}

		id, err := uuidStringFromPg(row.ID)
		if err != nil {
			return nil, err
		}
		events = append(events, OutboxClickEvent{
			ID:          id,
			Slug:        row.Slug,
			OccurredAt:  row.OccurredAt.Time.UTC(),
			TraceParent: outboxNullableTextValue(row.Traceparent),
			TraceState:  outboxNullableTextValue(row.Tracestate),
			Baggage:     outboxNullableTextValue(row.Baggage),
			Attempts:    int(row.Attempts),
		})
	}

	return events, nil
}

func (r *ClickOutboxRepository) MarkSent(ctx context.Context, id string, workerID string) error {
	pgID, err := parsePgUUID(id)
	if err != nil {
		return err
	}
	rows, err := r.queries.MarkOutboxSent(ctx, sqlc.MarkOutboxSentParams{
		ID:              pgID,
		ProcessingOwner: toOutboxNullableText(workerID),
		SentAt:          toOutboxTimestamptz(time.Now().UTC()),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOutboxEventNotOwned
	}
	return nil
}

func (r *ClickOutboxRepository) MarkRetry(
	ctx context.Context,
	id string,
	workerID string,
	lastError string,
	nextAttemptAt time.Time,
) error {
	pgID, err := parsePgUUID(id)
	if err != nil {
		return err
	}
	rows, err := r.queries.MarkOutboxRetry(ctx, sqlc.MarkOutboxRetryParams{
		ID:              pgID,
		ProcessingOwner: toOutboxNullableText(workerID),
		LastError:       toOutboxNullableText(lastError),
		NextAttemptAt:   toOutboxTimestamptz(nextAttemptAt),
		UpdatedAt:       toOutboxTimestamptz(time.Now().UTC()),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrOutboxEventNotOwned
	}
	return nil
}

func toOutboxNullableText(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{
		String: v,
		Valid:  true,
	}
}

func toOutboxTimestamptz(v time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{
		Time:  v.UTC(),
		Valid: true,
	}
}

func outboxNullableTextValue(v pgtype.Text) string {
	if !v.Valid {
		return ""
	}
	return v.String
}

func parsePgUUID(raw string) (pgtype.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(raw))
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}, nil
}

func uuidStringFromPg(v pgtype.UUID) (string, error) {
	if !v.Valid {
		return "", errors.New("invalid outbox uuid")
	}
	id := uuid.UUID(v.Bytes)
	return id.String(), nil
}
