package httputils

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/IgorGrieder/encurtador-url/internal/constants"
	appvalidation "github.com/IgorGrieder/encurtador-url/internal/infrastructure/validation"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

const InternalServerErrMsg = "error processing the request, try again"

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

type ErrorResponse struct {
	Error   string            `json:"error"`
	Details map[string]string `json:"details,omitempty"`
}

type SuccessResponse struct {
	Data any `json:"data"`
}

func ValidateStruct(s any) error {
	return appvalidation.Validate(s)
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

// WriteJSON writes a JSON response with the given status code
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteAPIResponse writes an API response with metadata
func WriteAPIResponse(w http.ResponseWriter, r *http.Request, status int, data any) {
	correlationID := GetCorrelationID(r)

	w.Header().Set(CorrelationIDHeader, correlationID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := APIResponse{
		ResponseTime:  time.Now().UTC(),
		CorrelationId: correlationID,
		Data:          data,
	}

	json.NewEncoder(w).Encode(response)
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

// WriteError writes an error response without requiring an http.Request
func WriteError(w http.ResponseWriter, apiErr constants.APIError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(apiErr.Status)

	response := APIResponse{
		ResponseTime: time.Now().UTC(),
		Error:        apiErr.Code,
		Message:      apiErr.Message,
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

// Legacy functions kept for backward compatibility

func RespondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := SuccessResponse{Data: data}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode json response", "error", err)
	}
}

func RespondError(w http.ResponseWriter, status int, message string, details map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := ErrorResponse{
		Error:   message,
		Details: details,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

func RespondValidationError(w http.ResponseWriter, err error) {
	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		details := make(map[string]string)
		for _, e := range validationErrs {
			details[e.Field()] = e.Error()
		}
		RespondError(w, http.StatusBadRequest, "Validation failed", details)
		return
	}
	RespondError(w, http.StatusBadRequest, "Validation failed", nil)
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body", nil)
		return err
	}
	return nil
}
