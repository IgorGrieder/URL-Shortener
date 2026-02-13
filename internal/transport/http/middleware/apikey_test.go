package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAPIKeyMiddleware_OpenMode(t *testing.T) {
	// No keys configured → open mode, all requests pass through
	mw := APIKeyMiddleware(nil)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("open mode: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyMiddleware_ValidKey(t *testing.T) {
	mw := APIKeyMiddleware([]string{"secret-key-1"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "secret-key-1")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("valid key: got status %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestAPIKeyMiddleware_MissingHeader(t *testing.T) {
	mw := APIKeyMiddleware([]string{"secret-key-1"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("missing header: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyMiddleware_WrongKey(t *testing.T) {
	mw := APIKeyMiddleware([]string{"secret-key-1"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(APIKeyHeader, "wrong-key")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestAPIKeyMiddleware_EmptyKeysSlice(t *testing.T) {
	// Empty slice of keys → open mode
	mw := APIKeyMiddleware([]string{})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("empty keys: got status %d, want %d", rec.Code, http.StatusOK)
	}
}
