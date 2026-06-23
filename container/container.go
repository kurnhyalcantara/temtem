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

	// scaffold:example:start
	examplev1 "github.com/kurnhyalcantara/probopass/gen/go/probopass/example/v1"
	// scaffold:example:end
	// scaffold:redis:start
	redislib "github.com/redis/go-redis/v9"
	// scaffold:redis:end
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/kurnhyalcantara/kingler/pkg/middleware"
	platgrpc "github.com/kurnhyalcantara/kingler/pkg/platform/grpc"
	"github.com/kurnhyalcantara/kingler/pkg/platform/logger"
	"github.com/kurnhyalcantara/kingler/pkg/platform/postgres"

	// scaffold:redis:start
	"github.com/kurnhyalcantara/kingler/pkg/platform/redis"
	// scaffold:redis:end
	// scaffold:telemetry:start
	"github.com/kurnhyalcantara/kingler/pkg/platform/telemetry"
	// scaffold:telemetry:end
	// scaffold:example:start
	platvalidator "github.com/kurnhyalcantara/kingler/pkg/platform/validator"
	// scaffold:example:end

	"github.com/kurnhyalcantara/temtem/config"
	// scaffold:example:start
	"github.com/kurnhyalcantara/temtem/internal/handler"
	"github.com/kurnhyalcantara/temtem/internal/repository"
	"github.com/kurnhyalcantara/temtem/internal/usecase"
	"github.com/kurnhyalcantara/temtem/internal/validator"
	// scaffold:example:end
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
	log := logger.New(logger.Config{
		Level:   cfg.Log.Level,
		Format:  cfg.Log.Format,
		Name:    cfg.App.Name,
		Version: cfg.App.Version,
		Env:     cfg.App.Env,
	})

	// scaffold:telemetry:start
	tel, err := telemetry.New(ctx, telemetry.Config{
		Name:         cfg.App.Name,
		Version:      cfg.App.Version,
		Env:          cfg.App.Env,
		Enabled:      cfg.Telemetry.Enabled,
		OTLPEndpoint: cfg.Telemetry.OTLPEndpoint,
		SampleRatio:  cfg.Telemetry.SampleRatio,
	})
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:telemetry:end

	pg, err := postgres.New(ctx, postgres.Config{
		DSN:             cfg.Postgres.DSN(),
		MaxConns:        cfg.Postgres.MaxConns,
		MinConns:        cfg.Postgres.MinConns,
		MaxConnLifetime: cfg.Postgres.MaxConnLifetime,
	})
	if err != nil {
		return nil, fmt.Errorf("container: %w", err)
	}

	// scaffold:redis:start
	rdb, err := redis.New(ctx, redis.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		pg.Close()
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:redis:end

	// scaffold:example:start
	baseValidator := platvalidator.New()

	// Example feature: repository -> usecase -> handler.
	// scaffold:redis:start
	exampleRepo := repository.NewRedisCache(
		repository.NewPostgres(pg),
		rdb,
		cfg.Redis.CacheTTL,
		log,
	)
	// scaffold:redis:end
	// scaffold:redis:else:start
	// scaffold> exampleRepo := repository.NewPostgres(pg)
	// scaffold:redis:else:end
	exampleUsecase := usecase.New(exampleRepo)
	exampleHandler := handler.NewHandler(exampleUsecase, validator.New(baseValidator))
	// scaffold:example:end

	// Interceptor chain, outermost first.
	grpcServer, healthServer := platgrpc.NewServer(
		middleware.RequestID(),
		middleware.Recovery(log),
		middleware.Logging(log),
		middleware.AppError(),
	)
	// scaffold:example:start
	examplev1.RegisterExampleServiceServer(grpcServer, exampleHandler)
	// scaffold:example:end
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
	// scaffold:example:start
	if err := handler.RegisterREST(ctx, gatewayMux, gatewayConn); err != nil {
		pg.Close()
		// scaffold:redis:start
		_ = rdb.Close()
		// scaffold:redis:end
		_ = gatewayConn.Close()
		return nil, fmt.Errorf("container: %w", err)
	}
	// scaffold:example:end

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
