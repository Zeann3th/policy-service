package cache

import (
	"context"
	"errors"
	"time"
)

var ErrMiss = errors.New("cache miss")

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string) error
	SetWithTTL(ctx context.Context, key string, value string, ttl time.Duration) error
	TrySet(ctx context.Context, key string, value string) (bool, error)
	TrySetWithTTL(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Evict(ctx context.Context, key string) (bool, error)
	EvictAll(ctx context.Context) error
	Increment(ctx context.Context, key string) (int64, error)
}
