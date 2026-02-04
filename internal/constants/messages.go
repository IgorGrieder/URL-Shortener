package constants

// Error messages used in API responses.
// These are the human-readable messages returned in the "message" field.
const (
	// Common messages
	MsgInvalidRequestBody = "Invalid request body"
	MsgInternalError      = "An internal error occurred"
	MsgNotFound           = "Resource not found"
	MsgUnauthorized       = "Unauthorized"
	MsgRateLimited        = "Rate limit exceeded"

	// Shortener-specific messages
	MsgInvalidURL   = "Invalid URL (must be http or https)"
	MsgLinkNotFound = "Link not found"
	MsgLinkExpired  = "Link expired"
)
