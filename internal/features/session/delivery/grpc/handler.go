// Package grpc is the session feature's gRPC inbound adapter. It validates
// requests, maps them to dtos, delegates to the usecase, and maps results
// back to transport types.
package grpc

import (
	"context"

	sessionv1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/session/v1"

	"github.com/kurnhyalcantara/temtem/internal/features/session/dto"
	"github.com/kurnhyalcantara/temtem/internal/features/session/mapper"
	"github.com/kurnhyalcantara/temtem/internal/features/session/usecase"
	"github.com/kurnhyalcantara/temtem/internal/features/session/validator"
)

// ProtectedMethods lists the RPCs that require a valid access token; it is
// consumed by the auth interceptor during wiring.
var ProtectedMethods = map[string]bool{
	sessionv1.SessionService_GetSession_FullMethodName:    true,
	sessionv1.SessionService_RevokeSession_FullMethodName: true,
}

type Handler struct {
	sessionv1.UnimplementedSessionServiceServer

	usecase   usecase.Usecase
	validator *validator.Validator
}

func NewHandler(uc usecase.Usecase, val *validator.Validator) *Handler {
	return &Handler{usecase: uc, validator: val}
}

func (h *Handler) CreateSession(ctx context.Context, req *sessionv1.CreateSessionRequest) (*sessionv1.CreateSessionResponse, error) {
	userAgent, ipAddress := clientInfo(ctx)
	in := mapper.ToCreateSessionInput(req, userAgent, ipAddress)
	if err := h.validator.CreateSession(in); err != nil {
		return nil, err
	}

	out, err := h.usecase.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return mapper.ToCreateSessionResponse(out), nil
}

func (h *Handler) GetSession(ctx context.Context, req *sessionv1.GetSessionRequest) (*sessionv1.GetSessionResponse, error) {
	in := dto.GetSessionInput{SessionID: req.GetSessionId()}
	if err := h.validator.GetSession(in); err != nil {
		return nil, err
	}

	s, err := h.usecase.Get(ctx, in)
	if err != nil {
		return nil, err
	}
	return &sessionv1.GetSessionResponse{Session: mapper.ToProtoSession(s)}, nil
}

func (h *Handler) RefreshSession(ctx context.Context, req *sessionv1.RefreshSessionRequest) (*sessionv1.RefreshSessionResponse, error) {
	userAgent, ipAddress := clientInfo(ctx)
	in := mapper.ToRefreshSessionInput(req, userAgent, ipAddress)
	if err := h.validator.RefreshSession(in); err != nil {
		return nil, err
	}

	out, err := h.usecase.Refresh(ctx, in)
	if err != nil {
		return nil, err
	}
	return mapper.ToRefreshSessionResponse(out), nil
}

func (h *Handler) RevokeSession(ctx context.Context, req *sessionv1.RevokeSessionRequest) (*sessionv1.RevokeSessionResponse, error) {
	in := dto.RevokeSessionInput{SessionID: req.GetSessionId()}
	if err := h.validator.RevokeSession(in); err != nil {
		return nil, err
	}

	if err := h.usecase.Revoke(ctx, in); err != nil {
		return nil, err
	}
	return &sessionv1.RevokeSessionResponse{}, nil
}
