// Package registry attaches every feature's delivery adapters to the
// transport servers. Adding a feature means adding one line per transport
// here and exposing its constructors in quiver/provider.
package registry

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	sessionv1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/session/v1"
	grpclib "google.golang.org/grpc"

	sessiongrpc "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/grpc"
	sessionrest "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/rest"
)

// Handlers groups every feature's gRPC handler for registration.
type Handlers struct {
	Session *sessiongrpc.Handler
}

// GRPC registers all feature services on the gRPC server.
func GRPC(srv *grpclib.Server, h Handlers) {
	sessionv1.RegisterSessionServiceServer(srv, h.Session)
}

// Gateway registers all feature REST translations on the gateway mux.
func Gateway(ctx context.Context, mux *runtime.ServeMux, conn *grpclib.ClientConn) error {
	return sessionrest.Register(ctx, mux, conn)
}

// ProtectedMethods merges every feature's auth-required RPCs for the auth
// interceptor.
func ProtectedMethods() map[string]bool {
	protected := map[string]bool{}
	for method := range sessiongrpc.ProtectedMethods {
		protected[method] = true
	}
	return protected
}
