package events

// ClickRecorded is emitted when a redirect click is accepted by the API.
type ClickRecorded struct {
	EventID    string `json:"eventId"`
	Slug       string `json:"slug"`
	OccurredAt string `json:"occurredAt"`
}
