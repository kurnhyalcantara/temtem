// Package provider holds one constructor per dependency. Providers contain
// wiring only — construction order and business rules live elsewhere.
//
// Infrastructure is initialized by the shared kingler platform packages;
// providers translate this service's config structs into kingler's
// functional options.
package provider

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	redislib "github.com/redis/go-redis/v9"

	platjwt "github.com/kurnhyalcantara/kingler/pkg/platform/jwt"
	"github.com/kurnhyalcantara/kingler/pkg/platform/logger"
	"github.com/kurnhyalcantara/kingler/pkg/platform/postgres"
	"github.com/kurnhyalcantara/kingler/pkg/platform/redis"
	"github.com/kurnhyalcantara/kingler/pkg/platform/telemetry"
	platvalidator "github.com/kurnhyalcantara/kingler/pkg/platform/validator"

	"github.com/kurnhyalcantara/temtem/config"
)

func Logger(cfg *config.Config) *slog.Logger {
	return logger.New(
		logger.WithLevel(cfg.Log.Level),
		logger.WithFormat(cfg.Log.Format),
		logger.WithService(cfg.App.Name, cfg.App.Version, cfg.App.Env),
	)
}

func Postgres(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	return postgres.New(ctx,
		postgres.WithDSN(cfg.Postgres.DSN()),
		postgres.WithMaxConns(cfg.Postgres.MaxConns),
		postgres.WithMinConns(cfg.Postgres.MinConns),
		postgres.WithMaxConnLifetime(cfg.Postgres.MaxConnLifetime),
	)
}

func Redis(ctx context.Context, cfg *config.Config) (*redislib.Client, error) {
	return redis.New(ctx,
		redis.WithAddr(cfg.Redis.Addr),
		redis.WithPassword(cfg.Redis.Password),
		redis.WithDB(cfg.Redis.DB),
	)
}

func Telemetry(ctx context.Context, cfg *config.Config) (*telemetry.Telemetry, error) {
	return telemetry.New(ctx,
		telemetry.WithService(cfg.App.Name, cfg.App.Version, cfg.App.Env),
		telemetry.WithEnabled(cfg.Telemetry.Enabled),
		telemetry.WithOTLPEndpoint(cfg.Telemetry.OTLPEndpoint),
		telemetry.WithSampleRatio(cfg.Telemetry.SampleRatio),
	)
}

func TokenManager(cfg *config.Config) *platjwt.TokenManager {
	return platjwt.NewTokenManager(
		platjwt.WithSecret(cfg.JWT.Secret),
		platjwt.WithIssuer(cfg.JWT.Issuer),
		platjwt.WithAccessTTL(cfg.JWT.AccessTTL),
		platjwt.WithRefreshTTL(cfg.JWT.RefreshTTL),
	)
}

func Validator() *platvalidator.Validator {
	return platvalidator.New()
}
