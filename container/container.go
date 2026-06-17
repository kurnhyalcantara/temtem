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
	sessionv1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/session/v1"
	redislib "github.com/redis/go-redis/v9"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kurnhyalcantara/kingler/pkg/middleware"
	platgrpc "github.com/kurnhyalcantara/kingler/pkg/platform/grpc"
	platjwt "github.com/kurnhyalcantara/kingler/pkg/platform/jwt"
	"github.com/kurnhyalcantara/kingler/pkg/platform/logger"
	"github.com/kurnhyalcantara/kingler/pkg/platform/postgres"
	"github.com/kurnhyalcantara/kingler/pkg/platform/redis"
	"github.com/kurnhyalcantara/kingler/pkg/platform/telemetry"
	platvalidator "github.com/kurnhyalcantara/kingler/pkg/platform/validator"

	"github.com/kurnhyalcantara/temtem/config"
	sessiongrpc "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/grpc"
	sessionrest "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/rest"
	"github.com/kurnhyalcantara/temtem/internal/features/session/repository"
	"github.com/kurnhyalcantara/temtem/internal/features/session/usecase"
	"github.com/kurnhyalcantara/temtem/internal/features/session/validator"
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
	log := logger.New(
		logger.WithLevel(cfg.Log.Level),
		logger.WithFormat(cfg.Log.Format),
		logger.WithService(cfg.App.Name, cfg.App.Version, cfg.App.Env),
	)

	tel, err := telemetry.New(ctx,
		telemetry.WithService(cfg.App.Name, cfg.App.Version, cfg.App.Env),
		telemetry.WithEnabled(cfg.Telemetry.Enabled),
		telemetry.WithOTLPEndpoint(cfg.Telemetry.OTLPEndpoint),
		telemetry.WithSampleRatio(cfg.Telemetry.SampleRatio),
	)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	pg, err := postgres.New(ctx,
		postgres.WithDSN(cfg.Postgres.DSN()),
		postgres.WithMaxConns(cfg.Postgres.MaxConns),
		postgres.WithMinConns(cfg.Postgres.MinConns),
		postgres.WithMaxConnLifetime(cfg.Postgres.MaxConnLifetime),
	)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	rdb, err := redis.New(ctx,
		redis.WithAddr(cfg.Redis.Addr),
		redis.WithPassword(cfg.Redis.Password),
		redis.WithDB(cfg.Redis.DB),
	)
	if err != nil {
		pg.Close()
		return nil, fmt.Errorf("container: %w", err)
	}

	// Platform components.
	tokens := platjwt.NewTokenManager(
		platjwt.WithSecret(cfg.JWT.Secret),
		platjwt.WithIssuer(cfg.JWT.Issuer),
		platjwt.WithAccessTTL(cfg.JWT.AccessTTL),
		platjwt.WithRefreshTTL(cfg.JWT.RefreshTTL),
	)
	baseValidator := platvalidator.New()

	// Session feature: repository -> usecase -> handler. The Redis cache TTL
	// tracks the access-token TTL: a cached session can never outlive the
	// token that references it.
	sessionRepo := repository.NewRedisCache(
		repository.NewPostgres(pg),
		rdb,
		cfg.JWT.AccessTTL,
		log,
	)
	sessionUsecase := usecase.New(sessionRepo, tokens)
	sessionHandler := sessiongrpc.NewHandler(sessionUsecase, validator.New(baseValidator))

	// Interceptor chain, outermost first. AppError must wrap Auth so that
	// errors returned by the auth interceptor are also mapped to statuses.
	grpcServer, healthServer := platgrpc.NewServer(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.Logging(log),
		middleware.AppError(),
		middleware.Auth(tokens, protectedMethods()),
	)
	sessionv1.RegisterSessionServiceServer(grpcServer, sessionHandler)
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	gatewayMux := platgrpc.NewGatewayMux(middleware.GatewayOptions()...)
	gatewayConn, err := platgrpc.NewLoopbackClient(cfg.Server.GRPCPort)
	if err != nil {
		pg.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("container: %w", err)
	}
	if err := sessionrest.Register(ctx, gatewayMux, gatewayConn); err != nil {
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

// protectedMethods merges every feature's auth-required RPCs for the auth
// interceptor.
func protectedMethods() map[string]bool {
	protected := map[string]bool{}
	for method := range sessiongrpc.ProtectedMethods {
		protected[method] = true
	}
	return protected
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
