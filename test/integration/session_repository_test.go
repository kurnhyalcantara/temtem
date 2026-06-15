//go:build integration

// Integration tests run against real Postgres/Redis, e.g. the
// docker-compose services:
//
//	make docker-up && make migrate-up && make test-integration
//
// Connection settings come from the usual TEMTEM_* environment variables.
package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kurnhyalcantara/temtem/config"
	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
	"github.com/kurnhyalcantara/temtem/internal/features/session/repository"
	"github.com/kurnhyalcantara/temtem/platform/postgres"
)

func TestPostgresSessionRepository(t *testing.T) {
	// Default to the docker-compose credentials so the test runs out of the
	// box; real environments override via TEMTEM_* variables.
	if os.Getenv("TEMTEM_JWT__SECRET") == "" {
		t.Setenv("TEMTEM_JWT__SECRET", "integration-test")
	}
	if os.Getenv("TEMTEM_POSTGRES__PASSWORD") == "" {
		t.Setenv("TEMTEM_POSTGRES__PASSWORD", "temtem")
	}

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := postgres.New(ctx, cfg.Postgres)
	if err != nil {
		t.Fatalf("connect postgres (are the compose services up and migrated?): %v", err)
	}
	defer pool.Close()

	repo := repository.NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	s := &domain.Session{
		ID:               uuid.NewString(),
		UserID:           "it-user",
		RefreshTokenHash: uuid.NewString(),
		UserAgent:        "integration-test",
		IPAddress:        "127.0.0.1",
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
	}

	if err := repo.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM sessions WHERE id = $1`, s.ID)
	})

	got, err := repo.GetByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != s.UserID || !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", got, s)
	}

	byHash, err := repo.GetByRefreshTokenHash(ctx, s.RefreshTokenHash)
	if err != nil {
		t.Fatalf("GetByRefreshTokenHash: %v", err)
	}
	if byHash.ID != s.ID {
		t.Fatalf("hash lookup returned %s, want %s", byHash.ID, s.ID)
	}

	got.Revoke(now)
	if err := repo.Update(ctx, got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	updated, err := repo.GetByID(ctx, s.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if !updated.IsRevoked() {
		t.Fatal("revocation not persisted")
	}

	if _, err := repo.GetByID(ctx, uuid.NewString()); err != domain.ErrNotFound {
		t.Fatalf("expected domain.ErrNotFound, got %v", err)
	}
}
