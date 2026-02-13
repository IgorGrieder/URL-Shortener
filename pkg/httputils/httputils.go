package httputils

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/constants"
	"github.com/google/uuid"
)

const CorrelationIDHeader = "X-Correlation-Id"

// APIResponse wraps all API responses with metadata
type APIResponse struct {
	ResponseTime  time.Time `json:"responseTime" example:"2024-01-15T10:30:00Z"`
	CorrelationId string    `json:"correlationId" example:"550e8400-e29b-41d4-a716-446655440000"`
	Code          string    `json:"code,omitempty" example:"ITEM_CREATED"`
	Data          any       `json:"data,omitempty"`
	Error         string    `json:"error,omitempty" example:"INVALID_REQUEST"`
	Message       string    `json:"message,omitempty" example:"Request processed successfully"`
}

type SuccessResponse struct {
	Data any `json:"data"`
}

// GetCorrelationID extracts the correlation ID from the request header
// If not present, generates a new UUID v4
func GetCorrelationID(r *http.Request) string {
	correlationID := r.Header.Get(CorrelationIDHeader)
	if correlationID == "" {
		correlationID = uuid.New().String()
	}
	return correlationID
}

// WriteAPIError writes an error response with metadata using a predefined APIError
func WriteAPIError(w http.ResponseWriter, r *http.Request, apiErr constants.APIError) {
	correlationID := GetCorrelationID(r)

	w.Header().Set(CorrelationIDHeader, correlationID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Status)

	response := APIResponse{
		ResponseTime:  time.Now().UTC(),
		CorrelationId: correlationID,
		Error:         apiErr.Code,
		Message:       apiErr.Message,
	}

	json.NewEncoder(w).Encode(response)
}

// WriteAPISuccess writes a success response with metadata using a predefined APISuccess
func WriteAPISuccess(w http.ResponseWriter, r *http.Request, apiSuccess constants.APISuccess, data any) {
	correlationID := GetCorrelationID(r)

	w.Header().Set(CorrelationIDHeader, correlationID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiSuccess.Status)

	response := APIResponse{
		ResponseTime:  time.Now().UTC(),
		CorrelationId: correlationID,
		Code:          apiSuccess.Code,
		Data:          data,
	}

	json.NewEncoder(w).Encode(response)
}

func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := SuccessResponse{Data: data}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}
