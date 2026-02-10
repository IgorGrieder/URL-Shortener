package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/config"
	"github.com/IgorGrieder/encurtador-url/internal/constants"
	"github.com/IgorGrieder/encurtador-url/internal/infrastructure/logger"
	appvalidation "github.com/IgorGrieder/encurtador-url/internal/infrastructure/validation"
	"github.com/IgorGrieder/encurtador-url/internal/processing/links"
	"github.com/IgorGrieder/encurtador-url/internal/transport/http/middleware"
	"github.com/IgorGrieder/encurtador-url/pkg/httputils"
	"github.com/go-playground/validator/v10"
	"go.uber.org/zap"
)

type LinksHandler struct {
	cfg *config.Config
	svc *links.Service

	asyncClick   bool
	clickTimeout time.Duration
	fastRedirect bool
}

func NewLinksHandler(cfg *config.Config, svc *links.Service) *LinksHandler {
	return NewLinksHandlerWithOptions(cfg, svc, LinksHandlerOptions{
		AsyncClick:   true,
		ClickTimeout: 2 * time.Second,
		FastRedirect: true,
	})
}

type LinksHandlerOptions struct {
	AsyncClick   bool
	ClickTimeout time.Duration
	FastRedirect bool
}

func NewLinksHandlerWithOptions(cfg *config.Config, svc *links.Service, opts LinksHandlerOptions) *LinksHandler {
	if opts.ClickTimeout <= 0 {
		opts.ClickTimeout = 2 * time.Second
	}

	return &LinksHandler{
		cfg:          cfg,
		svc:          svc,
		asyncClick:   opts.AsyncClick,
		clickTimeout: opts.ClickTimeout,
		fastRedirect: opts.FastRedirect,
	}
}

type createLinkRequest struct {
	URL       string     `json:"url" validate:"required,notblank,http_url"`
	Notes     string     `json:"notes,omitempty"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty" validate:"omitempty,future"`
}

type createLinkResponse struct {
	Slug      string     `json:"slug"`
	URL       string     `json:"url"`
	ShortURL  string     `json:"shortUrl"`
	Notes     string     `json:"notes,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
}

func (h *LinksHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputils.WriteAPIError(w, r, constants.ErrInvalidRequestBody)
		return
	}
	if err := appvalidation.Validate(req); err != nil {
		apiErr := constants.ErrInvalidRequestBody
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			for _, e := range validationErrs {
				if e.Field() == "url" {
					apiErr = constants.ErrInvalidURL
					break
				}
				if e.Field() == "expiresAt" && e.Tag() == "future" {
					apiErr = apiErr.WithMessage("expiresAt must be in the future")
					break
				}
			}
		}
		httputils.WriteAPIError(w, r, apiErr)
		return
	}

	apiKey := r.Header.Get(middleware.APIKeyHeader)

	link, err := h.svc.CreateLink(r.Context(), links.CreateLinkInput{
		URL:       req.URL,
		Notes:     req.Notes,
		ExpiresAt: req.ExpiresAt,
		APIKey:    apiKey,
	})
	if err != nil {
		switch err {
		case links.ErrInvalidURL:
			httputils.WriteAPIError(w, r, constants.ErrInvalidURL)
		default:
			logger.Error("failed to create link", zap.Error(err))
			httputils.WriteAPIError(w, r, constants.ErrInternalError)
		}
		return
	}

	httputils.WriteAPISuccess(w, r, constants.SuccessLinkCreated, createLinkResponse{
		Slug:      link.Slug,
		URL:       link.URL,
		ShortURL:  strings.TrimRight(h.cfg.Shortener.BaseURL, "/") + "/" + link.Slug,
		Notes:     link.Notes,
		CreatedAt: link.CreatedAt,
		ExpiresAt: link.ExpiresAt,
	})
}

func (h *LinksHandler) Redirect(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	link, err := h.svc.Resolve(r.Context(), slug)
	if err != nil {
		switch err {
		case links.ErrNotFound:
			http.NotFound(w, r)
		case links.ErrExpired:
			w.WriteHeader(http.StatusGone)
		default:
			logger.Error("failed to resolve slug", zap.Error(err), zap.String("slug", slug))
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	if h.asyncClick {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), h.clickTimeout)
			defer cancel()
			if err := h.svc.RecordClick(ctx, slug); err != nil {
				logger.Warn("failed to record click", zap.Error(err), zap.String("slug", slug))
			}
		}()
	} else {
		_ = h.svc.RecordClick(r.Context(), slug)
	}

	if h.fastRedirect {
		w.Header().Set("Location", link.URL)
		w.WriteHeader(h.cfg.Shortener.RedirectStatus)
		return
	}
	http.Redirect(w, r, link.URL, h.cfg.Shortener.RedirectStatus)
}

type statsResponse struct {
	Slug  string             `json:"slug"`
	From  string             `json:"from"`
	To    string             `json:"to"`
	Daily []links.DailyCount `json:"daily"`
}

type statsQueryParams struct {
	From string `json:"from" validate:"required,datetime=2006-01-02"`
	To   string `json:"to" validate:"required,datetime=2006-01-02"`
}

func (h *LinksHandler) Stats(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")
	if err := appvalidation.Validate(statsQueryParams{From: fromRaw, To: toRaw}); err != nil {
		apiErr := constants.ErrInvalidRequestBody
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			for _, e := range validationErrs {
				if e.Tag() == "required" {
					apiErr = apiErr.WithMessage("from and to are required (YYYY-MM-DD)")
					break
				}
				if e.Field() == "from" && e.Tag() == "datetime" {
					apiErr = apiErr.WithMessage("invalid from (YYYY-MM-DD)")
					break
				}
				if e.Field() == "to" && e.Tag() == "datetime" {
					apiErr = apiErr.WithMessage("invalid to (YYYY-MM-DD)")
					break
				}
			}
		}
		httputils.WriteAPIError(w, r, apiErr)
		return
	}

	from, err := time.Parse(time.DateOnly, fromRaw)
	if err != nil {
		httputils.WriteAPIError(w, r, constants.ErrInvalidRequestBody.WithMessage("invalid from (YYYY-MM-DD)"))
		return
	}
	to, err := time.Parse(time.DateOnly, toRaw)
	if err != nil {
		httputils.WriteAPIError(w, r, constants.ErrInvalidRequestBody.WithMessage("invalid to (YYYY-MM-DD)"))
		return
	}

	daily, err := h.svc.GetStats(r.Context(), slug, from, to)
	if err != nil {
		switch err {
		case links.ErrNotFound:
			httputils.WriteAPIError(w, r, constants.ErrLinkNotFound)
		case links.ErrInvalidRange:
			httputils.WriteAPIError(w, r, constants.ErrInvalidRequestBody.WithMessage("from must be <= to"))
		default:
			logger.Error("failed to fetch stats", zap.Error(err), zap.String("slug", slug))
			httputils.WriteAPIError(w, r, constants.ErrInternalError)
		}
		return
	}

	httputils.WriteAPISuccess(w, r, constants.SuccessStatsFound, statsResponse{
		Slug:  slug,
		From:  from.Format(time.DateOnly),
		To:    to.Format(time.DateOnly),
		Daily: daily,
	})
}
