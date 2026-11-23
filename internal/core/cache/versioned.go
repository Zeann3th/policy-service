package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

const TTL = 15 * time.Minute

// Versioned caches values under keys that carry their namespace's
// generation, so a whole namespace is invalidated by bumping one counter.
type Versioned struct {
	logger      *slog.Logger
	cache       cacheport.Cache
	serviceName string
}

func NewVersioned(logger *slog.Logger, cache cacheport.Cache, serviceName string) *Versioned {
	return &Versioned{logger: logger, cache: cache, serviceName: serviceName}
}

func (v *Versioned) generation(ctx context.Context, namespace string) string {
	generation, err := v.cache.Get(ctx, GenerationKey(v.serviceName, namespace))
	if err == nil {
		return generation
	}
	if errors.Is(err, cacheport.ErrMiss) {
		return "0"
	}
	v.logger.Warn("get cache generation", "namespace", namespace, "error", err)
	return fmt.Sprintf("bypass-%d", time.Now().UnixNano())
}

func (v *Versioned) Key(ctx context.Context, namespace string, parts ...any) string {
	return Key(v.serviceName, append([]any{namespace, v.generation(ctx, namespace)}, parts...)...)
}

func (v *Versioned) Invalidate(ctx context.Context, namespaces ...string) {
	for _, namespace := range namespaces {
		if _, err := v.cache.Increment(ctx, GenerationKey(v.serviceName, namespace)); err != nil {
			v.logger.Warn("invalidate cache", "namespace", namespace, "error", err)
		}
	}
}

func Read[T any](ctx context.Context, v *Versioned, key string) (T, bool) {
	var zero T
	encoded, err := v.cache.Get(ctx, key)
	if err != nil {
		return zero, false
	}
	value, err := core.Decode[T](encoded)
	if err != nil {
		v.logger.Warn("decode cache", "key", key, "error", err)
		return zero, false
	}
	return *value, true
}

func Write[T any](ctx context.Context, v *Versioned, key string, value T) {
	WriteTTL(ctx, v, key, value, TTL)
}

// WriteTTL caches value for ttl instead of the default TTL.
func WriteTTL[T any](ctx context.Context, v *Versioned, key string, value T, ttl time.Duration) {
	encoded, err := core.Encode(&value)
	if err == nil {
		err = v.cache.SetWithTTL(ctx, key, encoded, ttl)
	}
	if err != nil {
		v.logger.Warn("write cache", "key", key, "error", err)
	}
}

// Fetch reads key through the cache: a hit is returned as is; on a miss,
// load runs and its value is written back unless load fails.
func Fetch[T any](ctx context.Context, v *Versioned, key string, load func() (T, error)) (T, error) {
	if cached, ok := Read[T](ctx, v, key); ok {
		return cached, nil
	}
	value, err := load()
	if err == nil {
		Write(ctx, v, key, value)
	}
	return value, err
}
