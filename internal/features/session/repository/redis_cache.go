package repository

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	domain "github.com/kurnhyalcantara/temtem/internal/domain/session"
)

// redisCache is a read-through cache decorator around another Repository.
// It demonstrates composing outbound adapters: the usecase still sees a
// single Repository.
type redisCache struct {
	next   Repository
	client *redis.Client
	ttl    time.Duration
	log    *slog.Logger
}

// NewRedisCache wraps `next` with read-through caching of GetByID lookups.
// Cache failures are logged and degrade to the underlying repository; they
// never fail the request.
func NewRedisCache(next Repository, client *redis.Client, ttl time.Duration, log *slog.Logger) Repository {
	return &redisCache{next: next, client: client, ttl: ttl, log: log}
}

func cacheKey(id string) string { return "session:" + id }

func (c *redisCache) Create(ctx context.Context, s *domain.Session) error {
	if err := c.next.Create(ctx, s); err != nil {
		return err
	}
	c.set(ctx, s)
	return nil
}

func (c *redisCache) GetByID(ctx context.Context, id string) (*domain.Session, error) {
	if data, err := c.client.Get(ctx, cacheKey(id)).Bytes(); err == nil {
		var s domain.Session
		if err := json.Unmarshal(data, &s); err == nil {
			return &s, nil
		}
	} else if err != redis.Nil {
		c.log.WarnContext(ctx, "session cache read failed", slog.String("error", err.Error()))
	}

	s, err := c.next.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	c.set(ctx, s)
	return s, nil
}

// GetByRefreshTokenHash always hits the primary store: refresh flows are
// rare and must observe the latest revocation state.
func (c *redisCache) GetByRefreshTokenHash(ctx context.Context, hash string) (*domain.Session, error) {
	return c.next.GetByRefreshTokenHash(ctx, hash)
}

func (c *redisCache) Update(ctx context.Context, s *domain.Session) error {
	if err := c.next.Update(ctx, s); err != nil {
		return err
	}
	if err := c.client.Del(ctx, cacheKey(s.ID)).Err(); err != nil {
		c.log.WarnContext(ctx, "session cache invalidation failed",
			slog.String("session_id", s.ID), slog.String("error", err.Error()))
	}
	return nil
}

func (c *redisCache) set(ctx context.Context, s *domain.Session) {
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	if err := c.client.Set(ctx, cacheKey(s.ID), data, c.ttl).Err(); err != nil {
		c.log.WarnContext(ctx, "session cache write failed", slog.String("error", err.Error()))
	}
}
