package mapper

import (
	"testing"
	"time"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
	"github.com/kurnhyalcantara/temtem/internal/features/session/dto"
)

func TestToProtoSession(t *testing.T) {
	created := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	revoked := created.Add(time.Hour)

	s := &domain.Session{
		ID:        "sid",
		UserID:    "uid",
		UserAgent: "ua",
		IPAddress: "1.2.3.4",
		ExpiresAt: created.Add(24 * time.Hour),
		RevokedAt: &revoked,
		CreatedAt: created,
	}

	pb := ToProtoSession(s)
	if pb.GetId() != s.ID || pb.GetUserId() != s.UserID {
		t.Fatalf("identity fields not mapped: %+v", pb)
	}
	if !pb.GetCreatedAt().AsTime().Equal(created) {
		t.Fatalf("created_at = %v, want %v", pb.GetCreatedAt().AsTime(), created)
	}
	if !pb.GetRevokedAt().AsTime().Equal(revoked) {
		t.Fatalf("revoked_at = %v, want %v", pb.GetRevokedAt().AsTime(), revoked)
	}
}

func TestToProtoSessionNilSafety(t *testing.T) {
	if ToProtoSession(nil) != nil {
		t.Fatal("nil session must map to nil")
	}
	active := &domain.Session{ID: "sid"}
	if ToProtoSession(active).GetRevokedAt() != nil {
		t.Fatal("active session must have nil revoked_at")
	}
}

func TestToCreateSessionResponse(t *testing.T) {
	out := &dto.SessionWithTokens{
		Session:      &domain.Session{ID: "sid"},
		AccessToken:  "access",
		RefreshToken: "refresh",
	}
	resp := ToCreateSessionResponse(out)
	if resp.GetSession().GetId() != "sid" || resp.GetAccessToken() != "access" || resp.GetRefreshToken() != "refresh" {
		t.Fatalf("response not mapped: %+v", resp)
	}
}
