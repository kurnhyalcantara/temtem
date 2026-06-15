package middleware

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/kurnhyalcantara/temtem/internal/constants"
	"github.com/kurnhyalcantara/temtem/pkg/ctxutil"
)

// RequestID reads the incoming x-request-id (or generates one), stores it in
// the context, and echoes it back in the response headers.
func RequestID() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := ""
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if vals := md.Get(constants.HeaderRequestID); len(vals) > 0 {
				requestID = vals[0]
			}
		}
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx = ctxutil.WithRequestID(ctx, requestID)
		_ = grpc.SetHeader(ctx, metadata.Pairs(constants.HeaderRequestID, requestID))
		return handler(ctx, req)
	}
}
