package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/daviroodrigues/esmaga-api/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailTemplateRepository struct {
	pool *pgxpool.Pool
}

func NewEmailTemplateRepository(pool *pgxpool.Pool) *EmailTemplateRepository {
	return &EmailTemplateRepository{pool: pool}
}

// SeedDefaults insere os templates padrão no boot caso ainda não existam no banco.
func (r *EmailTemplateRepository) SeedDefaults(ctx context.Context, templates []domain.EmailTemplate) error {
	const query = `
		INSERT INTO email_templates (slug, subject, html_body, text_body)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (slug) DO NOTHING
	`
	for _, t := range templates {
		if _, err := r.pool.Exec(ctx, query, t.Slug, t.Subject, t.HTMLBody, t.TextBody); err != nil {
			return fmt.Errorf("erro ao semear template %s: %w", t.Slug, err)
		}
	}
	return nil
}

func (r *EmailTemplateRepository) GetBySlug(ctx context.Context, slug string) (*domain.EmailTemplate, error) {
	const query = `
		SELECT slug, subject, html_body, text_body, updated_at
		FROM email_templates
		WHERE slug = $1
	`
	var t domain.EmailTemplate
	err := r.pool.QueryRow(ctx, query, slug).Scan(&t.Slug, &t.Subject, &t.HTMLBody, &t.TextBody, &t.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao buscar template %s: %w", slug, err)
	}
	return &t, nil
}
