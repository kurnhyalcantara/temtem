// Package mapper converts between transport types (the probopass contract),
// dtos, and domain entities. Mappers are pure functions; transport types must
// not leak past this package into usecases or the domain.
package mapper

import (
	sessionv1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/session/v1"
	"google.golang.org/protobuf/types/known/timestamppb"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
	"github.com/kurnhyalcantara/temtem/internal/features/session/dto"
)

func ToProtoSession(s *domain.Session) *sessionv1.Session {
	if s == nil {
		return nil
	}
	pb := &sessionv1.Session{
		Id:        s.ID,
		UserId:    s.UserID,
		UserAgent: s.UserAgent,
		IpAddress: s.IPAddress,
		ExpiresAt: timestamppb.New(s.ExpiresAt),
		CreatedAt: timestamppb.New(s.CreatedAt),
	}
	if s.RevokedAt != nil {
		pb.RevokedAt = timestamppb.New(*s.RevokedAt)
	}
	return pb
}

func ToCreateSessionResponse(out *dto.SessionWithTokens) *sessionv1.CreateSessionResponse {
	return &sessionv1.CreateSessionResponse{
		Session:      ToProtoSession(out.Session),
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
	}
}

func ToRefreshSessionResponse(out *dto.SessionWithTokens) *sessionv1.RefreshSessionResponse {
	return &sessionv1.RefreshSessionResponse{
		Session:      ToProtoSession(out.Session),
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
	}
}

func ToCreateSessionInput(req *sessionv1.CreateSessionRequest, userAgent, ipAddress string) dto.CreateSessionInput {
	return dto.CreateSessionInput{
		UserID:    req.GetUserId(),
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}
}

func ToRefreshSessionInput(req *sessionv1.RefreshSessionRequest, userAgent, ipAddress string) dto.RefreshSessionInput {
	return dto.RefreshSessionInput{
		RefreshToken: req.GetRefreshToken(),
		UserAgent:    userAgent,
		IPAddress:    ipAddress,
	}
}
