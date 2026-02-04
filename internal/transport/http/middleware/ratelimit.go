package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/constants"
	redisStorage "github.com/IgorGrieder/encurtador-url/internal/storage/redis"
	"github.com/IgorGrieder/encurtador-url/pkg/httputils"
)

// RedisFixedWindowLimiter enforces a simple counter per user per fixed time window.
// It also serves as a record of "how many requests this user made".
type RedisFixedWindowLimiter struct {
	store *redisStorage.FixedWindowLimiter
	limit int64
	now   func() time.Time
}

func NewRedisFixedWindowLimiter(store *redisStorage.FixedWindowLimiter, limitPerMinute int) *RedisFixedWindowLimiter {
	if limitPerMinute <= 0 {
		limitPerMinute = 60
	}
	return &RedisFixedWindowLimiter{
		store: store,
		limit: int64(limitPerMinute),
		now:   time.Now,
	}
}

func RateLimitMiddleware(limiter *RedisFixedWindowLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)
			ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
			defer cancel()

			count, err := limiter.store.Incr(ctx, key)
			if err != nil {
				// Fail open (MVP): do not block writes if Redis is temporarily unavailable.
				next.ServeHTTP(w, r)
				return
			}
			if count > limiter.limit {
				httputils.WriteAPIError(w, r, constants.ErrRateLimited)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitKey(r *http.Request) string {
	if apiKey := strings.TrimSpace(r.Header.Get(APIKeyHeader)); apiKey != "" {
		return "api_key:" + apiKey
	}

	// Fallback: use client IP (best-effort).
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return "ip:" + host
	}
	return "ip:unknown"
}
