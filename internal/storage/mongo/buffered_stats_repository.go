package mongo

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BufferedClickStatsOptions struct {
	QueueSize      int
	FlushInterval  time.Duration
	MaxBatchEvents int
	FlushTimeout   time.Duration
}

type BufferedClickStatsRepository struct {
	base         *ClickStatsRepository
	queue        chan clickEvent
	flushEvery   time.Duration
	maxBatch     int
	flushTimeout time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}

	dropped atomic.Int64
}

type clickEvent struct {
	slug string
	day  int32 // YYYYMMDD (UTC)
}

type daySlugKey struct {
	slug string
	day  int32
}

func NewBufferedClickStatsRepository(base *ClickStatsRepository, opts BufferedClickStatsOptions) *BufferedClickStatsRepository {
	const (
		defaultQueueSize      = 100_000
		defaultFlushInterval  = 250 * time.Millisecond
		defaultMaxBatchEvents = 50_000
		defaultFlushTimeout   = 2 * time.Second
	)

	if opts.QueueSize <= 0 {
		opts.QueueSize = defaultQueueSize
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = defaultFlushInterval
	}
	if opts.MaxBatchEvents <= 0 {
		opts.MaxBatchEvents = defaultMaxBatchEvents
	}
	if opts.FlushTimeout <= 0 {
		opts.FlushTimeout = defaultFlushTimeout
	}

	r := &BufferedClickStatsRepository{
		base:         base,
		queue:        make(chan clickEvent, opts.QueueSize),
		flushEvery:   opts.FlushInterval,
		maxBatch:     opts.MaxBatchEvents,
		flushTimeout: opts.FlushTimeout,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}

	go r.loop()
	return r
}

func (r *BufferedClickStatsRepository) IncDaily(ctx context.Context, slug string, at time.Time) error {
	if slug == "" {
		return nil
	}

	ev := clickEvent{
		slug: slug,
		day:  dayKey(at),
	}

	select {
	case r.queue <- ev:
		return nil
	default:
		r.dropped.Add(1)
		return nil
	}
}

func (r *BufferedClickStatsRepository) GetDaily(ctx context.Context, slug string, from, to time.Time) ([]links.DailyCount, error) {
	return r.base.GetDaily(ctx, slug, from, to)
}

func (r *BufferedClickStatsRepository) Dropped() int64 {
	return r.dropped.Load()
}

func (r *BufferedClickStatsRepository) Shutdown(ctx context.Context) error {
	r.stopOnce.Do(func() { close(r.stopCh) })

	select {
	case <-r.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *BufferedClickStatsRepository) loop() {
	defer close(r.doneCh)

	ticker := time.NewTicker(r.flushEvery)
	defer ticker.Stop()

	pending := make(map[daySlugKey]int64)
	var events int

	flush := func() {
		if events == 0 {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), r.flushTimeout)
		_ = r.flush(ctx, pending)
		cancel()

		pending = make(map[daySlugKey]int64)
		events = 0
	}

	drain := func() {
		for {
			select {
			case ev := <-r.queue:
				key := daySlugKey{slug: ev.slug, day: ev.day}
				pending[key]++
				events++
				if events >= r.maxBatch {
					flush()
				}
			default:
				return
			}
		}
	}

	for {
		select {
		case ev := <-r.queue:
			key := daySlugKey{slug: ev.slug, day: ev.day}
			pending[key]++
			events++
			if events >= r.maxBatch {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-r.stopCh:
			drain()
			flush()
			return
		}
	}
}

func (r *BufferedClickStatsRepository) flush(ctx context.Context, pending map[daySlugKey]int64) error {
	models := make([]mongo.WriteModel, 0, len(pending))

	for key, inc := range pending {
		date := dateStringFromDayKey(key.day)

		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"slug": key.slug, "date": date}).
			SetUpdate(bson.M{
				"$inc": bson.M{"count": inc},
				"$setOnInsert": bson.M{
					"slug": key.slug,
					"date": date,
				},
			}).
			SetUpsert(true),
		)
	}

	if len(models) == 0 {
		return nil
	}

	_, err := r.base.coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

func dayKey(t time.Time) int32 {
	y, m, d := t.UTC().Date()
	return int32(y*10000 + int(m)*100 + d)
}

func dateStringFromDayKey(day int32) string {
	y := int(day / 10000)
	m := int((day / 100) % 100)
	d := int(day % 100)

	var b [10]byte
	b[0] = byte('0' + (y/1000)%10)
	b[1] = byte('0' + (y/100)%10)
	b[2] = byte('0' + (y/10)%10)
	b[3] = byte('0' + y%10)
	b[4] = '-'
	b[5] = byte('0' + (m/10)%10)
	b[6] = byte('0' + m%10)
	b[7] = '-'
	b[8] = byte('0' + (d/10)%10)
	b[9] = byte('0' + d%10)
	return string(b[:])
}
