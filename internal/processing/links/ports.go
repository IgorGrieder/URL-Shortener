package links

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("link not found")
	ErrExpired      = errors.New("link expired")
	ErrInvalidURL   = errors.New("invalid url")
	ErrSlugTaken    = errors.New("slug taken")
	ErrInvalidRange = errors.New("invalid date range")
)

type LinkRepository interface {
	Insert(ctx context.Context, link *Link) error
	FindBySlug(ctx context.Context, slug string) (*Link, error)
	FindActiveBySlug(ctx context.Context, slug string, at time.Time) (*Link, error)
	FindActiveBySlugAndIncClick(ctx context.Context, slug string, at time.Time) (*Link, error)
	DeleteBySlug(ctx context.Context, slug string) (bool, error)
}

type StatsRepository interface {
	IncDaily(ctx context.Context, slug string, at time.Time) error
	GetDaily(ctx context.Context, slug string, from, to time.Time) ([]DailyCount, error)
	DeleteBySlug(ctx context.Context, slug string) error
}

type ClickOutboxRepository interface {
	EnqueueClick(ctx context.Context, slug string, occurredAt time.Time) error
}

type Slugger interface {
	Generate(length int) (string, error)
}
