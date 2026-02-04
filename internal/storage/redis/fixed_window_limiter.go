package redis

import (
	"context"
	"fmt"
	"time"
)

type FixedWindowLimiter struct {
	client *Client
	prefix string
	window time.Duration
	now    func() time.Time
}

func NewFixedWindowLimiter(client *Client, prefix string, window time.Duration) *FixedWindowLimiter {
	if prefix == "" {
		prefix = "rate"
	}
	if window <= 0 {
		window = time.Minute
	}
	return &FixedWindowLimiter{
		client: client,
		prefix: prefix,
		window: window,
		now:    time.Now,
	}
}

// Incr increments the counter for (key, current window) and returns the current count.
func (l *FixedWindowLimiter) Incr(ctx context.Context, key string) (int64, error) {
	if key == "" {
		key = "unknown"
	}

	windowSeconds := int64(l.window.Seconds())
	if windowSeconds <= 0 {
		windowSeconds = 60
	}

	now := l.now().UTC()
	bucket := now.Unix() / windowSeconds
	redisKey := fmt.Sprintf("%s:%s:%d", l.prefix, key, bucket)

	count, err := l.client.Incr(ctx, redisKey)
	if err != nil {
		return 0, err
	}

	// For cleanup only. Extending TTL doesn't change the "fixed window" behavior because the key includes bucket.
	_ = l.client.ExpireSeconds(ctx, redisKey, windowSeconds*2)

	return count, nil
}

