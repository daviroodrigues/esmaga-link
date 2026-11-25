package domain

import "time"

const (
	TemplateEmailVerification = "email_verification"
	TemplatePasswordReset     = "password_reset"
	TemplateAccountDeleted    = "account_deleted"
)

type EmailMessage struct {
	To             string            `json:"to"`
	Template       string            `json:"template"`
	Variables      map[string]string `json:"variables"`
	IdempotencyKey string            `json:"idempotency_key,omitempty"`
}

type EmailTemplate struct {
	Slug      string    `json:"slug"`
	Subject   string    `json:"subject"`
	HTMLBody  string    `json:"html_body"`
	TextBody  string    `json:"text_body"`
	UpdatedAt time.Time `json:"updated_at"`
}
