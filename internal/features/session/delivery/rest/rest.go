// Package rest is the session feature's REST inbound adapter: it registers
// the grpc-gateway translation for SessionService onto the shared mux.
package rest

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	sessionv1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/session/v1"
	"google.golang.org/grpc"
)

// Register wires SessionService's REST routes (from the proto http
// annotations) into the gateway mux, forwarding over the given connection.
func Register(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	if err := sessionv1.RegisterSessionServiceHandler(ctx, mux, conn); err != nil {
		return fmt.Errorf("session rest: register gateway handler: %w", err)
	}
	return nil
}
