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
	FindActiveBySlugAndIncClick(ctx context.Context, slug string, at time.Time) (*Link, error)
}

type StatsRepository interface {
	IncDaily(ctx context.Context, slug string, at time.Time) error
	GetDaily(ctx context.Context, slug string, from, to time.Time) ([]DailyCount, error)
}

type Slugger interface {
	Generate(length int) (string, error)
}
