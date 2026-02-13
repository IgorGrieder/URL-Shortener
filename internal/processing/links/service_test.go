package links

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- Hand-written mocks ---

type mockLinkRepo struct {
	insertFn       func(ctx context.Context, link *Link) error
	findBySlugFn   func(ctx context.Context, slug string) (*Link, error)
	findActiveFn   func(ctx context.Context, slug string, at time.Time) (*Link, error)
	deleteBySlugFn func(ctx context.Context, slug string) (bool, error)
}

func (m *mockLinkRepo) Insert(ctx context.Context, link *Link) error {
	return m.insertFn(ctx, link)
}
func (m *mockLinkRepo) FindBySlug(ctx context.Context, slug string) (*Link, error) {
	return m.findBySlugFn(ctx, slug)
}
func (m *mockLinkRepo) FindActiveBySlug(ctx context.Context, slug string, at time.Time) (*Link, error) {
	return m.findActiveFn(ctx, slug, at)
}
func (m *mockLinkRepo) FindActiveBySlugAndIncClick(ctx context.Context, slug string, at time.Time) (*Link, error) {
	return m.findActiveFn(ctx, slug, at)
}
func (m *mockLinkRepo) DeleteBySlug(ctx context.Context, slug string) (bool, error) {
	return m.deleteBySlugFn(ctx, slug)
}

type mockStatsRepo struct {
	getDailyFn    func(ctx context.Context, slug string, from, to time.Time) ([]DailyCount, error)
	deleteBySlugFn func(ctx context.Context, slug string) error
}

func (m *mockStatsRepo) IncDaily(context.Context, string, time.Time) error { return nil }
func (m *mockStatsRepo) GetDaily(ctx context.Context, slug string, from, to time.Time) ([]DailyCount, error) {
	return m.getDailyFn(ctx, slug, from, to)
}
func (m *mockStatsRepo) DeleteBySlug(ctx context.Context, slug string) error {
	return m.deleteBySlugFn(ctx, slug)
}

type mockOutboxRepo struct {
	enqueueFn func(ctx context.Context, slug string, at time.Time) error
}

func (m *mockOutboxRepo) EnqueueClick(ctx context.Context, slug string, at time.Time) error {
	return m.enqueueFn(ctx, slug, at)
}

type mockSlugger struct {
	slugs []string
	idx   int
}

func (m *mockSlugger) Generate(int) (string, error) {
	if m.idx >= len(m.slugs) {
		return "", errors.New("no more slugs")
	}
	s := m.slugs[m.idx]
	m.idx++
	return s, nil
}

// --- Tests for validateAndNormalizeURL ---

func TestValidateAndNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{"valid https", "https://example.com/path", "https://example.com/path", false},
		{"valid http", "http://example.com", "http://example.com", false},
		{"strips fragment", "https://example.com/page#section", "https://example.com/page", false},
		{"empty string", "", "", true},
		{"bad scheme ftp", "ftp://example.com", "", true},
		{"no scheme", "example.com", "", true},
		{"missing host", "https://", "", true},
		{"whitespace trimmed", "  https://example.com  ", "https://example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateAndNormalizeURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- Tests for dateOnly ---

func TestDateOnly(t *testing.T) {
	input := time.Date(2025, 6, 15, 14, 30, 45, 123, time.UTC)
	got := dateOnly(input)
	want := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("dateOnly(%v) = %v, want %v", input, got, want)
	}
}

// --- Tests for Service ---

func newTestService(lr *mockLinkRepo, sr *mockStatsRepo, or ClickOutboxRepository, sl *mockSlugger) *Service {
	svc := NewService(lr, sr, or, sl, 6)
	svc.now = func() time.Time {
		return time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	}
	return svc
}

func TestCreateLink_HappyPath(t *testing.T) {
	lr := &mockLinkRepo{
		insertFn: func(_ context.Context, _ *Link) error { return nil },
	}
	sr := &mockStatsRepo{}
	sl := &mockSlugger{slugs: []string{"abc123"}}

	svc := newTestService(lr, sr, nil, sl)

	link, err := svc.CreateLink(context.Background(), CreateLinkInput{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if link.Slug != "abc123" {
		t.Errorf("got slug %q, want %q", link.Slug, "abc123")
	}
	if link.URL != "https://example.com" {
		t.Errorf("got URL %q, want %q", link.URL, "https://example.com")
	}
}

func TestCreateLink_InvalidURL(t *testing.T) {
	svc := newTestService(&mockLinkRepo{}, &mockStatsRepo{}, nil, &mockSlugger{})

	_, err := svc.CreateLink(context.Background(), CreateLinkInput{URL: "not-a-url"})
	if !errors.Is(err, ErrInvalidURL) {
		t.Fatalf("expected ErrInvalidURL, got: %v", err)
	}
}

func TestCreateLink_SlugCollisionRetries(t *testing.T) {
	attempts := 0
	lr := &mockLinkRepo{
		insertFn: func(_ context.Context, _ *Link) error {
			attempts++
			if attempts <= 2 {
				return ErrSlugTaken
			}
			return nil
		},
	}
	sl := &mockSlugger{slugs: []string{"s1", "s2", "s3"}}

	svc := newTestService(lr, &mockStatsRepo{}, nil, sl)

	link, err := svc.CreateLink(context.Background(), CreateLinkInput{URL: "https://example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if link.Slug != "s3" {
		t.Errorf("got slug %q, want %q", link.Slug, "s3")
	}
	if attempts != 3 {
		t.Errorf("expected 3 insert attempts, got %d", attempts)
	}
}

func TestCreateLink_AllRetriesExhausted(t *testing.T) {
	lr := &mockLinkRepo{
		insertFn: func(_ context.Context, _ *Link) error { return ErrSlugTaken },
	}
	slugs := make([]string, 10)
	for i := range slugs {
		slugs[i] = "dup"
	}
	sl := &mockSlugger{slugs: slugs}

	svc := newTestService(lr, &mockStatsRepo{}, nil, sl)

	_, err := svc.CreateLink(context.Background(), CreateLinkInput{URL: "https://example.com"})
	if !errors.Is(err, ErrSlugTaken) {
		t.Fatalf("expected ErrSlugTaken after exhausting retries, got: %v", err)
	}
}

func TestGetLink_EmptySlug(t *testing.T) {
	svc := newTestService(&mockLinkRepo{}, &mockStatsRepo{}, nil, &mockSlugger{})

	_, err := svc.GetLink(context.Background(), "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestGetLink_DelegatesToRepo(t *testing.T) {
	want := &Link{Slug: "abc", URL: "https://example.com"}
	lr := &mockLinkRepo{
		findBySlugFn: func(_ context.Context, slug string) (*Link, error) {
			if slug == "abc" {
				return want, nil
			}
			return nil, ErrNotFound
		},
	}

	svc := newTestService(lr, &mockStatsRepo{}, nil, &mockSlugger{})

	got, err := svc.GetLink(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != want.Slug {
		t.Errorf("got slug %q, want %q", got.Slug, want.Slug)
	}
}

func TestResolve_EmptySlug(t *testing.T) {
	svc := newTestService(&mockLinkRepo{}, &mockStatsRepo{}, nil, &mockSlugger{})

	_, err := svc.Resolve(context.Background(), "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestResolve_DelegatesToRepo(t *testing.T) {
	want := &Link{Slug: "xyz", URL: "https://example.com"}
	lr := &mockLinkRepo{
		findActiveFn: func(_ context.Context, slug string, _ time.Time) (*Link, error) {
			return want, nil
		},
	}

	svc := newTestService(lr, &mockStatsRepo{}, nil, &mockSlugger{})

	got, err := svc.Resolve(context.Background(), "xyz")
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != want.URL {
		t.Errorf("got URL %q, want %q", got.URL, want.URL)
	}
}

func TestRecordClick_NilOutbox(t *testing.T) {
	svc := newTestService(&mockLinkRepo{}, &mockStatsRepo{}, nil, &mockSlugger{})

	if err := svc.RecordClick(context.Background(), "abc"); err != nil {
		t.Fatalf("nil outbox should be no-op, got: %v", err)
	}
}

func TestRecordClick_EmptySlug(t *testing.T) {
	called := false
	or := &mockOutboxRepo{
		enqueueFn: func(_ context.Context, _ string, _ time.Time) error {
			called = true
			return nil
		},
	}

	svc := newTestService(&mockLinkRepo{}, &mockStatsRepo{}, or, &mockSlugger{})

	if err := svc.RecordClick(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("expected no-op for empty slug")
	}
}

func TestGetStats_InvalidRange(t *testing.T) {
	lr := &mockLinkRepo{
		findBySlugFn: func(_ context.Context, _ string) (*Link, error) {
			return &Link{Slug: "abc"}, nil
		},
	}

	svc := newTestService(lr, &mockStatsRepo{}, nil, &mockSlugger{})

	from := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)

	_, err := svc.GetStats(context.Background(), "abc", from, to)
	if !errors.Is(err, ErrInvalidRange) {
		t.Fatalf("expected ErrInvalidRange, got: %v", err)
	}
}

func TestGetStats_GapFilling(t *testing.T) {
	lr := &mockLinkRepo{
		findBySlugFn: func(_ context.Context, _ string) (*Link, error) {
			return &Link{Slug: "abc"}, nil
		},
	}
	sr := &mockStatsRepo{
		getDailyFn: func(_ context.Context, _ string, _, _ time.Time) ([]DailyCount, error) {
			return []DailyCount{
				{Date: "2025-01-01", Count: 5},
				{Date: "2025-01-03", Count: 3},
			}, nil
		},
		deleteBySlugFn: func(_ context.Context, _ string) error { return nil },
	}

	svc := newTestService(lr, sr, nil, &mockSlugger{})

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	counts, err := svc.GetStats(context.Background(), "abc", from, to)
	if err != nil {
		t.Fatal(err)
	}

	if len(counts) != 3 {
		t.Fatalf("expected 3 days, got %d", len(counts))
	}

	// Day 1: 5 clicks
	if counts[0].Date != "2025-01-01" || counts[0].Count != 5 {
		t.Errorf("day 0: got %+v", counts[0])
	}
	// Day 2: gap filled with 0
	if counts[1].Date != "2025-01-02" || counts[1].Count != 0 {
		t.Errorf("day 1 (gap): got %+v", counts[1])
	}
	// Day 3: 3 clicks
	if counts[2].Date != "2025-01-03" || counts[2].Count != 3 {
		t.Errorf("day 2: got %+v", counts[2])
	}
}

func TestDeleteLink_NotFound(t *testing.T) {
	lr := &mockLinkRepo{
		deleteBySlugFn: func(_ context.Context, _ string) (bool, error) {
			return false, nil
		},
	}

	svc := newTestService(lr, &mockStatsRepo{}, nil, &mockSlugger{})

	err := svc.DeleteLink(context.Background(), "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestDeleteLink_EmptySlug(t *testing.T) {
	svc := newTestService(&mockLinkRepo{}, &mockStatsRepo{}, nil, &mockSlugger{})

	err := svc.DeleteLink(context.Background(), "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for empty slug, got: %v", err)
	}
}
