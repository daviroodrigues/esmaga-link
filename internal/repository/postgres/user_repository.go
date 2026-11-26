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

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (name, email, password_hash, role, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query, u.Name, u.Email, u.PasswordHash, u.Role, u.Status).
		Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("erro ao inserir usuário no postgres: %w", err)
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, name, email, password_hash, role, status, email_verified_at, last_login_at, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, email).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.EmailVerifiedAt, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao buscar usuário por email: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, name, email, password_hash, role, status, email_verified_at, last_login_at, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u domain.User
	err := r.pool.QueryRow(ctx, query, id).
		Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.Status, &u.EmailVerifiedAt, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao buscar usuário por id: %w", err)
	}
	return &u, nil
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users
		SET email_verified_at = NOW(), status = 'active', updated_at = NOW()
		WHERE id = $1
	`
	if _, err := r.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("erro ao confirmar e-mail do usuário: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := `UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, query, userID); err != nil {
		return fmt.Errorf("erro ao atualizar último login: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	if _, err := r.pool.Exec(ctx, query, passwordHash, userID); err != nil {
		return fmt.Errorf("erro ao atualizar senha: %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateName(ctx context.Context, userID uuid.UUID, name *string) error {
	query := `UPDATE users SET name = $1, updated_at = NOW() WHERE id = $2`
	if _, err := r.pool.Exec(ctx, query, name, userID); err != nil {
		return fmt.Errorf("erro ao atualizar nome do usuário: %w", err)
	}
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("erro ao deletar usuário: %w", err)
	}
	return nil
}
