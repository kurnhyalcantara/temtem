package middleware

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kurnhyalcantara/temtem/pkg/apperror"
)

// AppError translates *apperror.Error values returned by handlers/usecases
// into gRPC status errors. Unknown error types become Internal with a
// generic message so internals never leak to clients.
func AppError() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		if appErr, ok := errors.AsType[*apperror.Error](err); ok {
			return nil, status.Error(grpcCode(appErr.Code), appErr.Message)
		}
		if _, ok := status.FromError(err); ok {
			return nil, err
		}
		return nil, status.Error(codes.Internal, "internal error")
	}
}

func grpcCode(code apperror.Code) codes.Code {
	switch code {
	case apperror.CodeInvalidArgument:
		return codes.InvalidArgument
	case apperror.CodeUnauthenticated:
		return codes.Unauthenticated
	case apperror.CodePermissionDenied:
		return codes.PermissionDenied
	case apperror.CodeNotFound:
		return codes.NotFound
	case apperror.CodeConflict:
		return codes.AlreadyExists
	default:
		return codes.Internal
	}
}
