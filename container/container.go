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
	// scaffold:session:start
	sessionv1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/session/v1"
	// scaffold:session:end
	// scaffold:redis:start
	redislib "github.com/redis/go-redis/v9"
	// scaffold:redis:end
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kurnhyalcantara/kingler/pkg/middleware"
	platgrpc "github.com/kurnhyalcantara/kingler/pkg/platform/grpc"
	// scaffold:session:start
	platjwt "github.com/kurnhyalcantara/kingler/pkg/platform/jwt"
	// scaffold:session:end
	"github.com/kurnhyalcantara/kingler/pkg/platform/logger"
	"github.com/kurnhyalcantara/kingler/pkg/platform/postgres"
	// scaffold:redis:start
	"github.com/kurnhyalcantara/kingler/pkg/platform/redis"
	// scaffold:redis:end
	// scaffold:telemetry:start
	"github.com/kurnhyalcantara/kingler/pkg/platform/telemetry"
	// scaffold:telemetry:end
	// scaffold:session:start
	platvalidator "github.com/kurnhyalcantara/kingler/pkg/platform/validator"
	// scaffold:session:end

	"github.com/kurnhyalcantara/temtem/config"
	// scaffold:session:start
	sessiongrpc "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/grpc"
	sessionrest "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/rest"
	"github.com/kurnhyalcantara/temtem/internal/features/session/repository"
	"github.com/kurnhyalcantara/temtem/internal/features/session/usecase"
	"github.com/kurnhyalcantara/temtem/internal/features/session/validator"
	// scaffold:session:end
)

type Container struct {
	Config   *config.Config
	Logger   *slog.Logger
	Postgres *pgxpool.Pool
	// scaffold:redis:start
	Redis *redislib.Client
	// scaffold:redis:end
	// scaffold:telemetry:start
	Telemetry *telemetry.Telemetry
	// scaffold:telemetry:end

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

	// scaffold:telemetry:start
	tel, err := telemetry.New(ctx,
		telemetry.WithService(cfg.App.Name, cfg.App.Version, cfg.App.Env),
		telemetry.WithEnabled(cfg.Telemetry.Enabled),
		telemetry.WithOTLPEndpoint(cfg.Telemetry.OTLPEndpoint),
		telemetry.WithSampleRatio(cfg.Telemetry.SampleRatio),
	)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:telemetry:end

	pg, err := postgres.New(ctx,
		postgres.WithDSN(cfg.Postgres.DSN()),
		postgres.WithMaxConns(cfg.Postgres.MaxConns),
		postgres.WithMinConns(cfg.Postgres.MinConns),
		postgres.WithMaxConnLifetime(cfg.Postgres.MaxConnLifetime),
	)
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	// scaffold:redis:start
	rdb, err := redis.New(ctx,
		redis.WithAddr(cfg.Redis.Addr),
		redis.WithPassword(cfg.Redis.Password),
		redis.WithDB(cfg.Redis.DB),
	)
	if err != nil {
		pg.Close()
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:redis:end

	// scaffold:session:start
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
	// scaffold:redis:start
	sessionRepo := repository.NewRedisCache(
		repository.NewPostgres(pg),
		rdb,
		cfg.JWT.AccessTTL,
		log,
	)
	// scaffold:redis:end
	// scaffold:redis:else:start
	// scaffold> sessionRepo := repository.NewPostgres(pg)
	// scaffold:redis:else:end
	sessionUsecase := usecase.New(sessionRepo, tokens)
	sessionHandler := sessiongrpc.NewHandler(sessionUsecase, validator.New(baseValidator))
	// scaffold:session:end

	// Interceptor chain, outermost first. AppError must wrap Auth so that
	// errors returned by the auth interceptor are also mapped to statuses.
	grpcServer, healthServer := platgrpc.NewServer(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.Logging(log),
		middleware.AppError(),
		// scaffold:session:start
		middleware.Auth(tokens, protectedMethods()),
		// scaffold:session:end
	)
	// scaffold:session:start
	sessionv1.RegisterSessionServiceServer(grpcServer, sessionHandler)
	// scaffold:session:end
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	gatewayMux := platgrpc.NewGatewayMux(middleware.GatewayOptions()...)
	gatewayConn, err := platgrpc.NewLoopbackClient(cfg.Server.GRPCPort)
	if err != nil {
		pg.Close()
		// scaffold:redis:start
		_ = rdb.Close()
		// scaffold:redis:end
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:session:start
	if err := sessionrest.Register(ctx, gatewayMux, gatewayConn); err != nil {
		pg.Close()
		// scaffold:redis:start
		_ = rdb.Close()
		// scaffold:redis:end
		_ = gatewayConn.Close()
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:session:end

	return &Container{
		Config:   cfg,
		Logger:   log,
		Postgres: pg,
		// scaffold:redis:start
		Redis: rdb,
		// scaffold:redis:end
		// scaffold:telemetry:start
		Telemetry: tel,
		// scaffold:telemetry:end
		GRPCServer:   grpcServer,
		HealthServer: healthServer,
		GatewayMux:   gatewayMux,
		gatewayConn:  gatewayConn,
	}, nil
}

// scaffold:session:start
// protectedMethods merges every feature's auth-required RPCs for the auth
// interceptor.
func protectedMethods() map[string]bool {
	protected := map[string]bool{}
	for method := range sessiongrpc.ProtectedMethods {
		protected[method] = true
	}
	return protected
}

// scaffold:session:end

// Ready reports whether downstream dependencies are reachable; it backs the
// /readyz endpoint.
func (c *Container) Ready(ctx context.Context) error {
	if err := c.Postgres.Ping(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	// scaffold:redis:start
	if err := c.Redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	// scaffold:redis:end
	return nil
}

// Close releases resources in reverse dependency order. The gRPC server
// must already be stopped by the caller.
func (c *Container) Close(ctx context.Context) error {
	errs := []error{
		c.gatewayConn.Close(),
		// scaffold:redis:start
		c.Redis.Close(),
		// scaffold:redis:end
		// scaffold:telemetry:start
		c.Telemetry.Shutdown(ctx),
		// scaffold:telemetry:end
	}
	c.Postgres.Close()
	return errors.Join(errs...)
}
