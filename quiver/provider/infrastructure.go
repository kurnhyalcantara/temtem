// Package provider holds one constructor per dependency. Providers contain
// wiring only — construction order and business rules live elsewhere.
package provider

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	redislib "github.com/redis/go-redis/v9"

	"github.com/kurnhyalcantara/temtem/config"
	platjwt "github.com/kurnhyalcantara/temtem/platform/jwt"
	"github.com/kurnhyalcantara/temtem/platform/logger"
	"github.com/kurnhyalcantara/temtem/platform/postgres"
	"github.com/kurnhyalcantara/temtem/platform/redis"
	"github.com/kurnhyalcantara/temtem/platform/telemetry"
	platvalidator "github.com/kurnhyalcantara/temtem/platform/validator"
)

func Logger(cfg *config.Config) *slog.Logger {
	return logger.New(cfg.Log, cfg.App)
}

func Postgres(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	return postgres.New(ctx, cfg.Postgres)
}

func Redis(ctx context.Context, cfg *config.Config) (*redislib.Client, error) {
	return redis.New(ctx, cfg.Redis)
}

func Telemetry(ctx context.Context, cfg *config.Config) (*telemetry.Telemetry, error) {
	return telemetry.New(ctx, cfg.Telemetry, cfg.App)
}

func TokenManager(cfg *config.Config) *platjwt.TokenManager {
	return platjwt.NewTokenManager(cfg.JWT)
}

func Validator() *platvalidator.Validator {
	return platvalidator.New()
}
