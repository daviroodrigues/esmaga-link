package postgres

import (
	"context"
	"errors"
	"fmt"

	"esmaga-link/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TokenRepository struct {
	pool *pgxpool.Pool
}

func NewTokenRepository(pool *pgxpool.Pool) *TokenRepository {
	return &TokenRepository{pool: pool}
}

func (r *TokenRepository) CreateAuthToken(ctx context.Context, t *domain.AuthToken) error {
	const query = `
		INSERT INTO auth_tokens (user_id, token_hash, purpose, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	if err := r.pool.QueryRow(ctx, query, t.UserID, t.TokenHash, t.Purpose, t.ExpiresAt).
		Scan(&t.ID, &t.CreatedAt); err != nil {
		return fmt.Errorf("erro ao criar auth_token: %w", err)
	}
	return nil
}

func (r *TokenRepository) InvalidatePendingAuthTokens(ctx context.Context, userID uuid.UUID, purpose string) error {
	const query = `
		UPDATE auth_tokens
		SET used_at = NOW()
		WHERE user_id = $1 AND purpose = $2 AND used_at IS NULL
	`
	if _, err := r.pool.Exec(ctx, query, userID, purpose); err != nil {
		return fmt.Errorf("erro ao invalidar auth_tokens pendentes: %w", err)
	}
	return nil
}

func (r *TokenRepository) ConsumeAuthToken(ctx context.Context, tokenHash, purpose string) (*domain.AuthToken, error) {
	const query = `
		UPDATE auth_tokens
		SET used_at = NOW()
		WHERE token_hash = $1
		  AND purpose = $2
		  AND used_at IS NULL
		  AND expires_at > NOW()
		RETURNING id, user_id, token_hash, purpose, expires_at, used_at, created_at
	`
	var t domain.AuthToken
	err := r.pool.QueryRow(ctx, query, tokenHash, purpose).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.Purpose, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao consumir auth_token: %w", err)
	}
	return &t, nil
}

func (r *TokenRepository) CreateRefreshToken(ctx context.Context, t *domain.RefreshToken) error {
	const query = `
		INSERT INTO refresh_tokens (user_id, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	if err := r.pool.QueryRow(ctx, query, t.UserID, t.TokenHash, t.UserAgent, t.IPAddress, t.ExpiresAt).
		Scan(&t.ID, &t.CreatedAt); err != nil {
		return fmt.Errorf("erro ao criar refresh_token: %w", err)
	}
	return nil
}

func (r *TokenRepository) GetActiveRefreshToken(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	const query = `
		SELECT id, user_id, token_hash, user_agent, ip_address, expires_at, revoked_at, created_at
		FROM refresh_tokens
		WHERE token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`
	var t domain.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).
		Scan(&t.ID, &t.UserID, &t.TokenHash, &t.UserAgent, &t.IPAddress, &t.ExpiresAt, &t.RevokedAt, &t.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao buscar refresh_token: %w", err)
	}
	return &t, nil
}

func (r *TokenRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	const query = `UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, query, tokenHash); err != nil {
		return fmt.Errorf("erro ao revogar refresh_token: %w", err)
	}
	return nil
}

func (r *TokenRepository) RevokeAllUserRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	const query = `UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
	if _, err := r.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("erro ao revogar refresh_tokens do usuário: %w", err)
	}
	return nil
}
