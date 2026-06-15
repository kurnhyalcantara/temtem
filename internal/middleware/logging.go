package middleware

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/kurnhyalcantara/temtem/pkg/ctxutil"
)

// Logging emits one structured log line per RPC with method, code, duration,
// and request id.
func Logging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		attrs := []slog.Attr{
			slog.String("method", info.FullMethod),
			slog.String("code", status.Code(err).String()),
			slog.Duration("duration", time.Since(start)),
			slog.String("request_id", ctxutil.RequestIDFrom(ctx)),
		}
		if err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			log.LogAttrs(ctx, slog.LevelError, "rpc", attrs...)
			return resp, err
		}
		log.LogAttrs(ctx, slog.LevelInfo, "rpc", attrs...)
		return resp, nil
	}
}
