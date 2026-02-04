package constants

// Error codes used in API responses.
// These are the machine-readable codes returned in the "error" field.
const (
	// Common error codes
	CodeInvalidRequest = "INVALID_REQUEST"
	CodeInternalError  = "INTERNAL_ERROR"
	CodeForbidden      = "FORBIDDEN"
	CodeNotFound       = "NOT_FOUND"
	CodeUnauthorized   = "UNAUTHORIZED"
	CodeRateLimited    = "RATE_LIMITED"

	// Shortener-specific codes
	CodeInvalidURL   = "INVALID_URL"
	CodeLinkExpired  = "LINK_EXPIRED"
	CodeLinkNotFound = "LINK_NOT_FOUND"

	// Success codes
	CodeLinkCreated = "LINK_CREATED"
	CodeStatsFound  = "STATS_FOUND"
)
