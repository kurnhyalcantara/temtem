// Package grpc builds the gRPC server, the grpc-gateway mux, and the
// loopback client connection the gateway uses to reach the server.
package grpc

import (
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// NewServer builds a gRPC server with OTel instrumentation, the given unary
// interceptor chain, reflection, and a health service.
func NewServer(interceptors ...grpclib.UnaryServerInterceptor) (*grpclib.Server, *health.Server) {
	srv := grpclib.NewServer(
		grpclib.StatsHandler(otelgrpc.NewServerHandler()),
		grpclib.ChainUnaryInterceptor(interceptors...),
	)
	healthSrv := health.NewServer()
	healthpb.RegisterHealthServer(srv, healthSrv)
	reflection.Register(srv)
	return srv, healthSrv
}

// NewGatewayMux builds the grpc-gateway ServeMux. Behavioral options
// (error handlers, header matchers) are passed in by the composition root.
func NewGatewayMux(opts ...runtime.ServeMuxOption) *runtime.ServeMux {
	return runtime.NewServeMux(opts...)
}

// NewLoopbackClient returns a client connection to this process's own gRPC
// port, used by the gateway to forward translated REST calls.
func NewLoopbackClient(grpcPort int) (*grpclib.ClientConn, error) {
	conn, err := grpclib.NewClient(
		fmt.Sprintf("localhost:%d", grpcPort),
		grpclib.WithTransportCredentials(insecure.NewCredentials()),
		grpclib.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		return nil, fmt.Errorf("grpc: create loopback client: %w", err)
	}
	return conn, nil
}
