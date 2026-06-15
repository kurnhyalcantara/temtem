// Package usecase implements the session feature's application logic.
// It depends only on the domain, the repository port, dtos, and shared
// packages — never on transport (gen/) or infrastructure drivers.
package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
	"github.com/kurnhyalcantara/temtem/internal/features/session/dto"
	"github.com/kurnhyalcantara/temtem/internal/features/session/repository"
	"github.com/kurnhyalcantara/temtem/pkg/apperror"
	"github.com/kurnhyalcantara/temtem/pkg/ctxutil"
)

// TokenIssuer is the usecase's view of the JWT platform component, defined
// here (where it is consumed) so the usecase stays decoupled from platform.
type TokenIssuer interface {
	SignAccessToken(userID, sessionID string) (string, error)
	RefreshTTL() time.Duration
}

type Usecase interface {
	// Create starts a new session and issues a token pair.
	Create(ctx context.Context, in dto.CreateSessionInput) (*dto.SessionWithTokens, error)
	// Get returns the caller's session by id.
	Get(ctx context.Context, in dto.GetSessionInput) (*domain.Session, error)
	// Refresh rotates a refresh token: the old session is revoked and a new
	// one is created with a fresh token pair.
	Refresh(ctx context.Context, in dto.RefreshSessionInput) (*dto.SessionWithTokens, error)
	// Revoke invalidates the caller's session (logout).
	Revoke(ctx context.Context, in dto.RevokeSessionInput) error
}

type sessionUsecase struct {
	repo   repository.Repository
	tokens TokenIssuer
	now    func() time.Time
}

func New(repo repository.Repository, tokens TokenIssuer) Usecase {
	return &sessionUsecase{repo: repo, tokens: tokens, now: time.Now}
}

func (u *sessionUsecase) Create(ctx context.Context, in dto.CreateSessionInput) (*dto.SessionWithTokens, error) {
	return u.startSession(ctx, in.UserID, in.UserAgent, in.IPAddress)
}

func (u *sessionUsecase) Get(ctx context.Context, in dto.GetSessionInput) (*domain.Session, error) {
	s, err := u.authorizedSession(ctx, in.SessionID)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (u *sessionUsecase) Refresh(ctx context.Context, in dto.RefreshSessionInput) (*dto.SessionWithTokens, error) {
	now := u.now()

	current, err := u.repo.GetByRefreshTokenHash(ctx, hashRefreshToken(in.RefreshToken))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Deliberately indistinguishable from other invalid-token cases.
			return nil, apperror.Unauthenticated("invalid refresh token")
		}
		return nil, apperror.Internal(err)
	}
	if err := current.Refreshable(now); err != nil {
		return nil, apperror.Unauthenticated("invalid refresh token")
	}

	current.Revoke(now)
	if err := u.repo.Update(ctx, current); err != nil {
		return nil, apperror.Internal(err)
	}

	return u.startSession(ctx, current.UserID, in.UserAgent, in.IPAddress)
}

func (u *sessionUsecase) Revoke(ctx context.Context, in dto.RevokeSessionInput) error {
	s, err := u.authorizedSession(ctx, in.SessionID)
	if err != nil {
		return err
	}
	if s.IsRevoked() {
		return nil // idempotent logout
	}
	s.Revoke(u.now())
	if err := u.repo.Update(ctx, s); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// startSession creates and persists a session for userID and issues its
// token pair. Shared by Create and Refresh (rotation).
func (u *sessionUsecase) startSession(ctx context.Context, userID, userAgent, ipAddress string) (*dto.SessionWithTokens, error) {
	now := u.now()

	refreshToken, err := newRefreshToken()
	if err != nil {
		return nil, apperror.Internal(err)
	}

	s := &domain.Session{
		ID:               uuid.NewString(),
		UserID:           userID,
		RefreshTokenHash: hashRefreshToken(refreshToken),
		UserAgent:        userAgent,
		IPAddress:        ipAddress,
		ExpiresAt:        now.Add(u.tokens.RefreshTTL()),
		CreatedAt:        now,
	}
	if err := u.repo.Create(ctx, s); err != nil {
		return nil, apperror.Internal(err)
	}

	accessToken, err := u.tokens.SignAccessToken(s.UserID, s.ID)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	return &dto.SessionWithTokens{
		Session:      s,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// authorizedSession loads a session and enforces that the authenticated
// caller owns it.
func (u *sessionUsecase) authorizedSession(ctx context.Context, sessionID string) (*domain.Session, error) {
	identity, ok := ctxutil.IdentityFrom(ctx)
	if !ok {
		return nil, apperror.Unauthenticated("authentication required")
	}

	s, err := u.repo.GetByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, apperror.NotFound("session not found")
		}
		return nil, apperror.Internal(err)
	}
	if s.UserID != identity.UserID {
		return nil, apperror.New(apperror.CodePermissionDenied, "session belongs to another user")
	}
	return s, nil
}
