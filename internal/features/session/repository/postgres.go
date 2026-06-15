package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
)

type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgres returns the PostgreSQL-backed session repository.
func NewPostgres(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

const sessionColumns = `id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, revoked_at, created_at`

func (r *postgresRepository) Create(ctx context.Context, s *domain.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (`+sessionColumns+`)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.ID, s.UserID, s.RefreshTokenHash, s.UserAgent, s.IPAddress,
		s.ExpiresAt, s.RevokedAt, s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("session repository: create: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	return r.getOne(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE id = $1`, id)
}

func (r *postgresRepository) GetByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	return r.getOne(ctx, `SELECT `+sessionColumns+` FROM sessions WHERE refresh_token_hash = $1`, hash)
}

func (r *postgresRepository) getOne(ctx context.Context, query string, arg any) (*domain.Session, error) {
	var s domain.Session
	err := r.pool.QueryRow(ctx, query, arg).Scan(
		&s.ID, &s.UserID, &s.RefreshTokenHash, &s.UserAgent, &s.IPAddress,
		&s.ExpiresAt, &s.RevokedAt, &s.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("session repository: get: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) Update(ctx context.Context, s *domain.Session) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE sessions
		SET refresh_token_hash = $2, expires_at = $3, revoked_at = $4
		WHERE id = $1`,
		s.ID, s.RefreshTokenHash, s.ExpiresAt, s.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("session repository: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
