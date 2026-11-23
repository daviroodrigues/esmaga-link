package domain

import (
	"time"

	"github.com/google/uuid"
)

type ClickEvent struct {
	EventID    uuid.UUID `json:"event_id"`
	ShortCode  string    `json:"short_code"`
	ClickedAt  time.Time `json:"clicked_at"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	Referrer   string    `json:"referrer"`
	Country    string    `json:"country"`
	DeviceType string    `json:"device_type"`
}
