package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status" example:"ok"`
	Timestamp string `json:"timestamp" example:"2024-01-15T10:30:00Z"`
}

// HealthHandler handles health and metrics endpoints
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health returns the health status of the service
// @Summary      Health check
// @Description  Returns the health status of the API server
// @Tags         Health
// @Produce      json
// @Success      200  {object}  HealthResponse
// @Router       /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// Metrics returns Prometheus metrics
func (h *HealthHandler) Metrics() http.Handler {
	return promhttp.Handler()
}
