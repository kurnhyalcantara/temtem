package middleware

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/kurnhyalcantara/temtem/internal/constants"
	"github.com/kurnhyalcantara/temtem/pkg/apperror"
	"github.com/kurnhyalcantara/temtem/pkg/ctxutil"
	platjwt "github.com/kurnhyalcantara/temtem/platform/jwt"
)

// Auth verifies bearer access tokens for protected methods and injects the
// caller Identity into the context. Methods not present in `protected`
// (e.g. login, refresh) pass through unauthenticated.
func Auth(tokens *platjwt.TokenManager, protected map[string]bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !protected[info.FullMethod] {
			return handler(ctx, req)
		}

		token, err := bearerToken(ctx)
		if err != nil {
			return nil, err
		}
		claims, err := tokens.VerifyAccessToken(token)
		if err != nil {
			return nil, apperror.Unauthenticated("invalid or expired access token")
		}

		ctx = ctxutil.WithIdentity(ctx, ctxutil.Identity{
			UserID:    claims.UserID,
			SessionID: claims.SessionID,
		})
		return handler(ctx, req)
	}
}

func bearerToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", apperror.Unauthenticated("missing metadata")
	}
	vals := md.Get(constants.HeaderAuthorization)
	if len(vals) == 0 {
		return "", apperror.Unauthenticated("missing authorization header")
	}
	if !strings.HasPrefix(vals[0], constants.BearerPrefix) {
		return "", apperror.Unauthenticated("authorization header must use Bearer scheme")
	}
	return strings.TrimPrefix(vals[0], constants.BearerPrefix), nil
}
