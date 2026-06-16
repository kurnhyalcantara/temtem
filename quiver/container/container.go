// Package container is the application's composition root: Build wires
// configuration, infrastructure, repositories, usecases, handlers, and
// middleware into runnable servers; Close releases everything in reverse
// order.
package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	redislib "github.com/redis/go-redis/v9"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kurnhyalcantara/kingler/pkg/middleware"
	platgrpc "github.com/kurnhyalcantara/kingler/pkg/platform/grpc"
	"github.com/kurnhyalcantara/kingler/pkg/platform/telemetry"

	"github.com/kurnhyalcantara/temtem/config"
	"github.com/kurnhyalcantara/temtem/quiver/provider"
	"github.com/kurnhyalcantara/temtem/quiver/registry"
)

type Container struct {
	Config    *config.Config
	Logger    *slog.Logger
	Postgres  *pgxpool.Pool
	Redis     *redislib.Client
	Telemetry *telemetry.Telemetry

	GRPCServer   *grpclib.Server
	HealthServer *health.Server
	GatewayMux   *runtime.ServeMux

	gatewayConn *grpclib.ClientConn
}

// Build constructs the full application graph.
func Build(ctx context.Context, cfg *config.Config) (*Container, error) {
	log := provider.Logger(cfg)

	tel, err := provider.Telemetry(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	pg, err := provider.Postgres(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	rdb, err := provider.Redis(ctx, cfg)
	if err != nil {
		pg.Close()
		return nil, fmt.Errorf("container: %w", err)
	}

	// Platform components.
	tokens := provider.TokenManager(cfg)
	baseValidator := provider.Validator()

	// Session feature: repository -> usecase -> handler.
	sessionRepo := provider.SessionRepository(pg, rdb, cfg, log)
	sessionUsecase := provider.SessionUsecase(sessionRepo, tokens)
	sessionHandler := provider.SessionHandler(sessionUsecase, provider.SessionValidator(baseValidator))

	handlers := registry.Handlers{Session: sessionHandler}

	// Interceptor chain, outermost first. AppError must wrap Auth so that
	// errors returned by the auth interceptor are also mapped to statuses.
	grpcServer, healthServer := platgrpc.NewServer(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.Logging(log),
		middleware.AppError(),
		middleware.Auth(tokens, registry.ProtectedMethods()),
	)
	registry.GRPC(grpcServer, handlers)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	gatewayMux := platgrpc.NewGatewayMux(middleware.GatewayOptions()...)
	gatewayConn, err := platgrpc.NewLoopbackClient(cfg.Server.GRPCPort)
	if err != nil {
		pg.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("container: %w", err)
	}
	if err := registry.Gateway(ctx, gatewayMux, gatewayConn); err != nil {
		pg.Close()
		_ = rdb.Close()
		_ = gatewayConn.Close()
		return nil, fmt.Errorf("container: %w", err)
	}

	return &Container{
		Config:       cfg,
		Logger:       log,
		Postgres:     pg,
		Redis:        rdb,
		Telemetry:    tel,
		GRPCServer:   grpcServer,
		HealthServer: healthServer,
		GatewayMux:   gatewayMux,
		gatewayConn:  gatewayConn,
	}, nil
}

// Ready reports whether downstream dependencies are reachable; it backs the
// /readyz endpoint.
func (c *Container) Ready(ctx context.Context) error {
	if err := c.Postgres.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := c.Redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

// Close releases resources in reverse dependency order. The gRPC server
// must already be stopped by the caller.
func (c *Container) Close(ctx context.Context) error {
	errs := []error{
		c.gatewayConn.Close(),
		c.Redis.Close(),
		c.Telemetry.Shutdown(ctx),
	}
	c.Postgres.Close()
	return errors.Join(errs...)
}
