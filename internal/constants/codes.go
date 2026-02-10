package constants

// Error codes used in API responses.
// These are the machine-readable codes returned in the "error" field.
const (
	// Common error codes
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeInternalError  = "INTERNAL_ERROR"
	CodeUnauthorized   = "UNAUTHORIZED"

	// Shortener-specific codes
	CodeInvalidURL   = "INVALID_URL"
	CodeLinkNotFound = "LINK_NOT_FOUND"

	// Success codes
	CodeLinkCreated = "LINK_CREATED"
	CodeLinkDeleted = "LINK_DELETED"
	CodeStatsFound  = "STATS_FOUND"
)
