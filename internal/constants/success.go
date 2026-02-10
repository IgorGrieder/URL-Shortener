package constants

import "net/http"

// APISuccess represents a standardized API success response with code and HTTP status.
// Use these predefined success constants for consistent API responses across the application.
type APISuccess struct {
	Code   string
	Status int
}

// Link-related success responses
var (
	SuccessLinkCreated = APISuccess{
		Code:   CodeLinkCreated,
		Status: http.StatusCreated,
	}
	SuccessLinkDeleted = APISuccess{
		Code:   CodeLinkDeleted,
		Status: http.StatusOK,
	}
	SuccessStatsFound = APISuccess{
		Code:   CodeStatsFound,
		Status: http.StatusOK,
	}
)
