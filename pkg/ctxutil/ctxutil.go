// Package ctxutil carries request-scoped values through context with typed
// keys. It is transport-agnostic so usecases can depend on it.
package ctxutil

import "context"

type ctxKey int

const (
	identityKey ctxKey = iota
	requestIDKey
)

// Identity is the authenticated caller extracted from the access token.
type Identity struct {
	UserID    string
	SessionID string
}

func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey).(Identity)
	return id, ok
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
