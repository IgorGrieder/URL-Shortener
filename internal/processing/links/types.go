package links

import "time"

type Link struct {
	Slug      string
	URL       string
	Notes     string
	CreatedAt time.Time
	ExpiresAt *time.Time
	APIKey    string
	Clicks    int64
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type CreateLinkInput struct {
	URL       string
	Notes     string
	ExpiresAt *time.Time
	APIKey    string
}
