package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

type memoryCache struct {
	cacheStub
	values map[string]string
	getErr error
}

func (c *memoryCache) Get(_ context.Context, key string) (string, error) {
	if c.getErr != nil {
		return "", c.getErr
	}
	value, ok := c.values[key]
	if !ok {
		return "", cacheport.ErrMiss
	}
	return value, nil
}

func (c *memoryCache) SetWithTTL(_ context.Context, key, value string, _ time.Duration) error {
	c.values[key] = value
	return nil
}

func (c *memoryCache) Increment(_ context.Context, key string) (int64, error) {
	c.values[key] = "1"
	return 1, nil
}

func newTestVersioned(cache cacheport.Cache) *Versioned {
	return NewVersioned(slog.New(slog.NewTextHandler(io.Discard, nil)), cache, "policy-service")
}

func TestVersionedKeyCarriesGenerationAndInvalidateMissesOldKey(t *testing.T) {
	ctx := context.Background()
	cache := &memoryCache{values: map[string]string{}}
	versioned := newTestVersioned(cache)

	key := versioned.Key(ctx, NamespacePolicyTypes, "item", 1)
	if key != "policy-service:policy:types:0:item:1" {
		t.Fatalf("key = %q", key)
	}
	Write(ctx, versioned, key, 42)
	if value, ok := Read[int](ctx, versioned, key); !ok || value != 42 {
		t.Fatalf("Read() = %d, %v", value, ok)
	}

	versioned.Invalidate(ctx, NamespacePolicyTypes)
	newKey := versioned.Key(ctx, NamespacePolicyTypes, "item", 1)
	if newKey == key {
		t.Fatal("Invalidate() must change the namespace generation")
	}
	if _, ok := Read[int](ctx, versioned, newKey); ok {
		t.Fatal("Read() after Invalidate() must miss")
	}
}

func TestVersionedKeyBypassesCacheWhenGenerationFails(t *testing.T) {
	versioned := newTestVersioned(&memoryCache{values: map[string]string{}, getErr: errors.New("redis down")})

	key := versioned.Key(context.Background(), NamespacePolicies, "item", 1)
	if !strings.HasPrefix(key, "policy-service:policies:bypass-") {
		t.Fatalf("key = %q, want a bypass generation", key)
	}
}
