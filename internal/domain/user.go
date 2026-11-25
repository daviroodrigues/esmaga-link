package domain

import (
	"time"

	"github.com/google/uuid"
)

const (
	UserStatusPending   = "pending"
	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"

	TokenPurposeEmailVerification = "email_verification"
	TokenPurposePasswordReset     = "password_reset"
)

type User struct {
	ID              uuid.UUID  `json:"id"`
	Name            *string    `json:"name,omitempty"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	Role            string     `json:"role"`
	Status          string     `json:"status"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AuthToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	Purpose   string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	UserAgent string
	IPAddress string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}
