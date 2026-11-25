package domain

import (
	"time"

	"github.com/google/uuid"
)

type Link struct {
	ShortCode   string     `json:"short_code"`
	OriginalURL string     `json:"original_url"`
	UserID      *uuid.UUID `json:"user_id,omitempty"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
}

type CreateLinkRequest struct {
	OriginalURL    string `json:"original_url"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
}

type EditLinkRequest struct {
	OriginalURL string `json:"original_url"`
}

type DeleteAccountRequest struct {
	KeepLinksActive bool `json:"keep_links_active"`
}

type LinkAnalytics struct {
	ShortCode       string           `json:"short_code"`
	TotalClicks     int64            `json:"total_clicks"`
	ClicksByDate    map[string]int64 `json:"clicks_by_date"`
	ClicksByDevice  map[string]int64 `json:"clicks_by_device"`
	ClicksByCountry map[string]int64 `json:"clicks_by_country"`
	TopReferrers    map[string]int64 `json:"top_referrers"`
}
