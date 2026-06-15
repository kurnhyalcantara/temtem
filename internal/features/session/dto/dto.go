// Package dto defines the session feature's internal request/response
// structures, decoupling usecases from transport types.
package dto

import (
	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
)

type CreateSessionInput struct {
	UserID    string `validate:"required,max=64"`
	UserAgent string `validate:"max=512"`
	IPAddress string `validate:"max=64"`
}

type RefreshSessionInput struct {
	RefreshToken string `validate:"required"`
	UserAgent    string `validate:"max=512"`
	IPAddress    string `validate:"max=64"`
}

type GetSessionInput struct {
	SessionID string `validate:"required,uuid4"`
}

type RevokeSessionInput struct {
	SessionID string `validate:"required,uuid4"`
}

// SessionWithTokens is returned by Create and Refresh: the persisted session
// plus the freshly issued token pair (the only time the refresh token is
// visible in plaintext).
type SessionWithTokens struct {
	Session      *domain.Session
	AccessToken  string
	RefreshToken string
}
