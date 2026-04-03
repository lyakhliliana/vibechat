package cache

import (
	"context"
	"fmt"
	"time"

	"vibechat/internal/infrastructure/cache/mock"
	"vibechat/internal/infrastructure/cache/redis"
)

// cleanupInterval is how often the mock cache sweeps expired entries.
const cleanupInterval = 5 * time.Minute

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
	Ping(ctx context.Context) error
	Close() error
}

// New constructs the cache backend selected by cfg.
func New(ctx context.Context, cfg Config) (Cache, error) {
	switch cfg.Type {
	case TypeRedis:
		if cfg.Redis == nil {
			return nil, fmt.Errorf("cache: redis config is required when type is %q", TypeRedis)
		}
		return redis.New(*cfg.Redis)
	case TypeMock, "":
		return mock.NewWithCleanup(ctx, cleanupInterval), nil
	default:
		return nil, fmt.Errorf("cache: unknown type %q", cfg.Type)
	}
}
