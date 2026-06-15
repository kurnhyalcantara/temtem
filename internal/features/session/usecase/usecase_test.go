package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
	"github.com/kurnhyalcantara/temtem/internal/features/session/dto"
	"github.com/kurnhyalcantara/temtem/pkg/apperror"
	"github.com/kurnhyalcantara/temtem/pkg/ctxutil"
)

// fakeRepo is an in-memory Repository double.
type fakeRepo struct {
	byID map[string]*domain.Session
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byID: map[string]*domain.Session{}}
}

func (r *fakeRepo) Create(_ context.Context, s *domain.Session) error {
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}

func (r *fakeRepo) GetByID(_ context.Context, id string) (*domain.Session, error) {
	s, ok := r.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *fakeRepo) GetByRefreshTokenHash(_ context.Context, hash string) (*domain.Session, error) {
	for _, s := range r.byID {
		if s.RefreshTokenHash == hash {
			cp := *s
			return &cp, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeRepo) Update(_ context.Context, s *domain.Session) error {
	if _, ok := r.byID[s.ID]; !ok {
		return domain.ErrNotFound
	}
	cp := *s
	r.byID[s.ID] = &cp
	return nil
}

type fakeTokenIssuer struct{}

func (fakeTokenIssuer) SignAccessToken(userID, sessionID string) (string, error) {
	return "access-" + userID + "-" + sessionID, nil
}

func (fakeTokenIssuer) RefreshTTL() time.Duration { return 24 * time.Hour }

var testNow = time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)

func newTestUsecase(repo *fakeRepo) *sessionUsecase {
	uc := New(repo, fakeTokenIssuer{}).(*sessionUsecase)
	uc.now = func() time.Time { return testNow }
	return uc
}

func authCtx(userID, sessionID string) context.Context {
	return ctxutil.WithIdentity(context.Background(), ctxutil.Identity{
		UserID:    userID,
		SessionID: sessionID,
	})
}

func assertCode(t *testing.T, err error, want apperror.Code) {
	t.Helper()
	appErr, ok := errors.AsType[*apperror.Error](err)
	if !ok {
		t.Fatalf("expected *apperror.Error, got %v", err)
	}
	if appErr.Code != want {
		t.Fatalf("expected code %s, got %s (%v)", want, appErr.Code, err)
	}
}

func TestCreateIssuesTokensAndPersists(t *testing.T) {
	repo := newFakeRepo()
	uc := newTestUsecase(repo)

	out, err := uc.Create(context.Background(), dto.CreateSessionInput{
		UserID: "user-1", UserAgent: "ua", IPAddress: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("expected non-empty token pair")
	}

	stored, err := repo.GetByID(context.Background(), out.Session.ID)
	if err != nil {
		t.Fatalf("session not persisted: %v", err)
	}
	if stored.RefreshTokenHash == out.RefreshToken {
		t.Fatal("refresh token must be stored hashed, not in plaintext")
	}
	if got, want := stored.ExpiresAt, testNow.Add(24*time.Hour); !got.Equal(want) {
		t.Fatalf("expires_at = %v, want %v", got, want)
	}
}

func TestGetEnforcesOwnership(t *testing.T) {
	repo := newFakeRepo()
	uc := newTestUsecase(repo)

	out, _ := uc.Create(context.Background(), dto.CreateSessionInput{UserID: "user-1"})
	in := dto.GetSessionInput{SessionID: out.Session.ID}

	if _, err := uc.Get(context.Background(), in); err == nil {
		t.Fatal("expected error without identity")
	} else {
		assertCode(t, err, apperror.CodeUnauthenticated)
	}

	if _, err := uc.Get(authCtx("user-2", "other"), in); err == nil {
		t.Fatal("expected error for foreign session")
	} else {
		assertCode(t, err, apperror.CodePermissionDenied)
	}

	s, err := uc.Get(authCtx("user-1", out.Session.ID), in)
	if err != nil {
		t.Fatalf("Get as owner: %v", err)
	}
	if s.ID != out.Session.ID {
		t.Fatalf("got session %s, want %s", s.ID, out.Session.ID)
	}

	missing := dto.GetSessionInput{SessionID: "00000000-0000-4000-8000-000000000000"}
	if _, err := uc.Get(authCtx("user-1", out.Session.ID), missing); err == nil {
		t.Fatal("expected not found")
	} else {
		assertCode(t, err, apperror.CodeNotFound)
	}
}

func TestRefreshRotatesSession(t *testing.T) {
	repo := newFakeRepo()
	uc := newTestUsecase(repo)

	created, _ := uc.Create(context.Background(), dto.CreateSessionInput{UserID: "user-1"})

	rotated, err := uc.Refresh(context.Background(), dto.RefreshSessionInput{
		RefreshToken: created.RefreshToken,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rotated.Session.ID == created.Session.ID {
		t.Fatal("rotation must create a new session")
	}
	if rotated.RefreshToken == created.RefreshToken {
		t.Fatal("rotation must issue a new refresh token")
	}

	old, _ := repo.GetByID(context.Background(), created.Session.ID)
	if !old.IsRevoked() {
		t.Fatal("old session must be revoked after rotation")
	}

	// The consumed token can no longer be used.
	if _, err := uc.Refresh(context.Background(), dto.RefreshSessionInput{
		RefreshToken: created.RefreshToken,
	}); err == nil {
		t.Fatal("expected reuse of consumed refresh token to fail")
	} else {
		assertCode(t, err, apperror.CodeUnauthenticated)
	}
}

func TestRefreshRejectsUnknownToken(t *testing.T) {
	uc := newTestUsecase(newFakeRepo())

	_, err := uc.Refresh(context.Background(), dto.RefreshSessionInput{RefreshToken: "bogus"})
	if err == nil {
		t.Fatal("expected error")
	}
	assertCode(t, err, apperror.CodeUnauthenticated)
}

func TestRevokeIsIdempotent(t *testing.T) {
	repo := newFakeRepo()
	uc := newTestUsecase(repo)

	created, _ := uc.Create(context.Background(), dto.CreateSessionInput{UserID: "user-1"})
	ctx := authCtx("user-1", created.Session.ID)
	in := dto.RevokeSessionInput{SessionID: created.Session.ID}

	if err := uc.Revoke(ctx, in); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	stored, _ := repo.GetByID(context.Background(), created.Session.ID)
	if !stored.IsRevoked() {
		t.Fatal("session must be revoked")
	}

	if err := uc.Revoke(ctx, in); err != nil {
		t.Fatalf("second Revoke must be a no-op, got: %v", err)
	}
}
