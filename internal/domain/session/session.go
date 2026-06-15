// Package session is the session domain model. It is pure: stdlib only,
// no transport, infrastructure, or framework imports.
package session

import (
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("session not found")
	ErrExpired  = errors.New("session expired")
	ErrRevoked  = errors.New("session revoked")
)

// Session is an authenticated user session. The refresh token itself is
// never stored; only its hash is kept for lookup during rotation.
type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	UserAgent        string
	IPAddress        string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	CreatedAt        time.Time
}

func (s *Session) IsRevoked() bool { return s.RevokedAt != nil }

func (s *Session) IsExpired(now time.Time) bool { return now.After(s.ExpiresAt) }

// Refreshable reports whether the session may be rotated at `now`.
func (s *Session) Refreshable(now time.Time) error {
	if s.IsRevoked() {
		return ErrRevoked
	}
	if s.IsExpired(now) {
		return ErrExpired
	}
	return nil
}

// Revoke marks the session revoked at `now`. Revoking twice keeps the
// original revocation time.
func (s *Session) Revoke(now time.Time) {
	if s.RevokedAt == nil {
		s.RevokedAt = &now
	}
}
