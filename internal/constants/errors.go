package constants

import "net/http"

// APIError represents a standardized API error with code, message, and HTTP status.
// Use these predefined errors for consistent API responses across the application.
type APIError struct {
	Code    string
	Message string
	Status  int
}

// WithMessage returns a copy of the APIError with a custom message.
// Useful for validation errors or other dynamic messages.
func (e APIError) WithMessage(message string) APIError {
	return APIError{
		Code:    e.Code,
		Message: message,
		Status:  e.Status,
	}
}

// Common errors - shared across multiple modules
var (
	ErrInvalidRequestBody = APIError{
		Code:    CodeInvalidRequest,
		Message: MsgInvalidRequestBody,
		Status:  http.StatusBadRequest,
	}
	ErrInternalError = APIError{
		Code:    CodeInternalError,
		Message: MsgInternalError,
		Status:  http.StatusInternalServerError,
	}
)

var (
	ErrUnauthorized = APIError{
		Code:    CodeUnauthorized,
		Message: MsgUnauthorized,
		Status:  http.StatusUnauthorized,
	}

	// Shortener-specific errors
	ErrInvalidURL = APIError{
		Code:    CodeInvalidURL,
		Message: MsgInvalidURL,
		Status:  http.StatusBadRequest,
	}
	ErrLinkNotFound = APIError{
		Code:    CodeLinkNotFound,
		Message: MsgLinkNotFound,
		Status:  http.StatusNotFound,
	}
)
