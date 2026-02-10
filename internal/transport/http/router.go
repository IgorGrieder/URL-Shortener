package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/config"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/telemetry"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"github.com/IgorGrieder/encurtador-url/internal/transport/http/middleware"
	"github.com/IgorGrieder/encurtador-url/pkg/httputils"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var spanNames = map[string]string{
	"GET /health":                 "health",
	"GET /metrics":                "metrics",
	"POST /api/links":             "links.create",
	"GET /api/links/{slug}/stats": "links.stats",
	"GET /{slug}":                 "links.redirect",
}

type RouterOptions struct {
	EnableCORS    bool
	EnableLogging bool
	EnableMetrics bool

	LinksHandlerOptions LinksHandlerOptions
}

func DefaultRouterOptions() RouterOptions {
	return RouterOptions{
		EnableCORS:    true,
		EnableLogging: true,
		EnableMetrics: true,
		LinksHandlerOptions: LinksHandlerOptions{
			AsyncClick:   true,
			ClickTimeout: 2 * time.Second,
			FastRedirect: false,
		},
	}
}

func NewRouter(cfg *config.Config, linkService *links.Service) http.Handler {
	return NewRouterWithOptions(cfg, linkService, DefaultRouterOptions())
}

func NewRouterWithOptions(cfg *config.Config, linkService *links.Service, opts RouterOptions) http.Handler {
	mux := http.NewServeMux()

	healthHandler := NewHealthHandler()
	linksHandler := NewLinksHandlerWithOptions(cfg, linkService, opts.LinksHandlerOptions)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		httputils.RespondJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().Format(time.RFC3339),
		})
	})
	mux.Handle("GET /metrics", healthHandler.Metrics())

	createMiddlewares := []func(http.Handler) http.Handler{
		middleware.APIKeyMiddleware(cfg.Security.APIKeys),
	}

	mux.Handle("POST /api/links", middleware.Chain(
		http.HandlerFunc(linksHandler.Create),
		createMiddlewares...,
	))

	mux.HandleFunc("GET /api/links/{slug}/stats", linksHandler.Stats)
	mux.HandleFunc("GET /{slug}", linksHandler.Redirect)

	var innerHandler http.Handler = mux
	if opts.EnableCORS {
		innerHandler = middleware.CORSMiddleware(innerHandler)
	}
	if opts.EnableLogging {
		innerHandler = middleware.LoggingMiddleware(innerHandler)
	}
	if opts.EnableMetrics {
		innerHandler = middleware.MetricsMiddleware(innerHandler)
	}

	otelOptions := []otelhttp.Option{
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			key := r.Method + " " + r.Pattern
			if name, ok := spanNames[key]; ok {
				return name
			}
			if r.Pattern != "" {
				return r.Pattern
			}
			path := strings.TrimSpace(r.URL.Path)
			if path == "" {
				path = "/"
			}
			return path
		}),
	}

	if telemetry.TracerProvider != nil {
		otelOptions = append(otelOptions, otelhttp.WithTracerProvider(telemetry.TracerProvider))
	}

	return otelhttp.NewHandler(innerHandler, cfg.App.Name, otelOptions...)
}
