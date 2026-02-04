package middleware

import (
	"net/http"
	"strings"

	"github.com/IgorGrieder/encurtador-url/internal/constants"
	"github.com/IgorGrieder/encurtador-url/pkg/httputils"
)

const APIKeyHeader = "X-API-Key"

func APIKeyMiddleware(allowedKeys []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, k := range allowedKeys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		allowed[k] = struct{}{}
	}

	// If no keys are configured, run open (MVP convenience).
	if len(allowed) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := strings.TrimSpace(r.Header.Get(APIKeyHeader))
			if apiKey == "" {
				httputils.WriteAPIError(w, r, constants.ErrUnauthorized)
				return
			}
			if _, ok := allowed[apiKey]; !ok {
				httputils.WriteAPIError(w, r, constants.ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

