package constants

// Error messages used in API responses.
// These are the human-readable messages returned in the "message" field.
const (
	// Common messages
	MsgInvalidRequestBody = "Invalid request body"
	MsgInternalError      = "An internal error occurred"
	MsgUnauthorized       = "Unauthorized"

	// Shortener-specific messages
	MsgInvalidURL   = "Invalid URL (must be http or https)"
	MsgLinkNotFound = "Link not found"
)
