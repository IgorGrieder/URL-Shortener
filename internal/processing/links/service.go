package links

import (
	"context"
	"net/url"
	"strings"
	"time"
)

type Service struct {
	linkRepo   LinkRepository
	statsRepo  StatsRepository
	slugger    Slugger
	slugLength int
	now        func() time.Time
}

func NewService(linkRepo LinkRepository, statsRepo StatsRepository, slugger Slugger, slugLength int) *Service {
	if slugLength <= 0 {
		slugLength = 6
	}

	return &Service{
		linkRepo:   linkRepo,
		statsRepo:  statsRepo,
		slugger:    slugger,
		slugLength: slugLength,
		now:        time.Now,
	}
}

func (s *Service) CreateLink(ctx context.Context, in CreateLinkInput) (*Link, error) {
	normalizedURL, err := validateAndNormalizeURL(in.URL)
	if err != nil {
		return nil, ErrInvalidURL
	}

	link := &Link{
		URL:       normalizedURL,
		Notes:     strings.TrimSpace(in.Notes),
		CreatedAt: s.now().UTC(),
		ExpiresAt: in.ExpiresAt,
		APIKey:    strings.TrimSpace(in.APIKey),
	}

	const maxAttempts = 10
	for range maxAttempts {
		slug, err := s.slugger.Generate(s.slugLength)
		if err != nil {
			return nil, err
		}
		link.Slug = slug

		if err := s.linkRepo.Insert(ctx, link); err != nil {
			if err == ErrSlugTaken {
				continue
			}
			return nil, err
		}

		return link, nil
	}

	return nil, ErrSlugTaken
}

func (s *Service) GetLink(ctx context.Context, slug string) (*Link, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return nil, ErrNotFound
	}

	link, err := s.linkRepo.FindBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	return link, nil
}

func (s *Service) Resolve(ctx context.Context, slug string) (*Link, error) {
	link, err := s.GetLink(ctx, slug)
	if err != nil {
		return nil, err
	}

	if link.ExpiresAt != nil && s.now().UTC().After(link.ExpiresAt.UTC()) {
		return nil, ErrExpired
	}

	return link, nil
}

func (s *Service) RecordClick(ctx context.Context, slug string) error {
	if strings.TrimSpace(slug) == "" {
		return nil
	}
	return s.statsRepo.IncDaily(ctx, slug, s.now().UTC())
}

func (s *Service) GetStats(ctx context.Context, slug string, from, to time.Time) ([]DailyCount, error) {
	link, err := s.GetLink(ctx, slug)
	if err != nil {
		return nil, err
	}
	_ = link

	from = from.UTC()
	to = to.UTC()
	if to.Before(from) {
		return nil, ErrInvalidRange
	}

	counts, err := s.statsRepo.GetDaily(ctx, slug, from, to)
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]int64, len(counts))
	for _, c := range counts {
		byDate[c.Date] = c.Count
	}

	out := make([]DailyCount, 0, int(to.Sub(from).Hours()/24)+1)
	for day := dateOnly(from); !day.After(dateOnly(to)); day = day.AddDate(0, 0, 1) {
		ds := day.Format(time.DateOnly)
		out = append(out, DailyCount{
			Date:  ds,
			Count: byDate[ds],
		})
	}

	return out, nil
}

func validateAndNormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrInvalidURL
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrInvalidURL
	}
	if strings.TrimSpace(u.Host) == "" {
		return "", ErrInvalidURL
	}

	u.Fragment = ""
	return u.String(), nil
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
