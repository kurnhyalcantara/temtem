package provider

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	redislib "github.com/redis/go-redis/v9"

	"github.com/kurnhyalcantara/temtem/config"
	sessiongrpc "github.com/kurnhyalcantara/temtem/internal/features/session/delivery/grpc"
	"github.com/kurnhyalcantara/temtem/internal/features/session/repository"
	"github.com/kurnhyalcantara/temtem/internal/features/session/usecase"
	"github.com/kurnhyalcantara/temtem/internal/features/session/validator"
	platjwt "github.com/kurnhyalcantara/temtem/platform/jwt"
	platvalidator "github.com/kurnhyalcantara/temtem/platform/validator"
)

// SessionRepository composes the PostgreSQL store with the Redis
// read-through cache. The cache TTL tracks the access-token TTL: a cached
// session can never outlive the token that references it.
func SessionRepository(pool *pgxpool.Pool, cache *redislib.Client, cfg *config.Config, log *slog.Logger) repository.Repository {
	return repository.NewRedisCache(
		repository.NewPostgres(pool),
		cache,
		cfg.JWT.AccessTTL,
		log,
	)
}

func SessionUsecase(repo repository.Repository, tokens *platjwt.TokenManager) usecase.Usecase {
	return usecase.New(repo, tokens)
}

func SessionValidator(v *platvalidator.Validator) *validator.Validator {
	return validator.New(v)
}

func SessionHandler(uc usecase.Usecase, val *validator.Validator) *sessiongrpc.Handler {
	return sessiongrpc.NewHandler(uc, val)
}
