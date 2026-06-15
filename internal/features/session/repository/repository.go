// Package repository defines the session feature's outbound port and its
// adapters. A repository is any outbound dependency abstraction — it may be
// backed by PostgreSQL, Redis, another gRPC/HTTP service, a third-party API,
// or a message broker; it is not limited to database access.
package repository

import (
	"context"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
)

// Repository is the outbound port consumed by the session usecase.
// Implementations: NewPostgres (primary store) and NewRedisCache
// (read-through cache decorator).
type Repository interface {
	Create(ctx context.Context, s *domain.Session) error
	GetByID(ctx context.Context, id string) (*domain.Session, error)
	GetByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error)
	// Update persists mutations of an existing session (e.g. revocation).
	Update(ctx context.Context, s *domain.Session) error
}
