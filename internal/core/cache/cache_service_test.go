package cache

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
)

type cacheStub struct {
	evicted     []string
	incremented []string
}

func (c *cacheStub) Get(context.Context, string) (string, error) { return "", nil }
func (c *cacheStub) Set(context.Context, string, string) error   { return nil }
func (c *cacheStub) SetWithTTL(context.Context, string, string, time.Duration) error {
	return nil
}
func (c *cacheStub) TrySet(context.Context, string, string) (bool, error) { return false, nil }
func (c *cacheStub) TrySetWithTTL(context.Context, string, string, time.Duration) (bool, error) {
	return false, nil
}
func (c *cacheStub) Evict(_ context.Context, key string) (bool, error) {
	c.evicted = append(c.evicted, key)
	return true, nil
}
func (c *cacheStub) EvictAll(context.Context) error { return nil }
func (c *cacheStub) Increment(_ context.Context, key string) (int64, error) {
	c.incremented = append(c.incremented, key)
	return 1, nil
}

func newTestService(cache *cacheStub) Service {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(logger, cache, "policy-service")
}

func TestReloadEvictsOnlyServiceOwnedKey(t *testing.T) {
	cache := &cacheStub{}
	response, err := newTestService(cache).Reload(context.Background(), ReloadRequest{
		Key:       "policy:config:0:CU0001:VND:1",
		UpdatedBy: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !response.Evicted || !reflect.DeepEqual(cache.evicted, []string{"policy-service:policy:config:0:CU0001:VND:1"}) {
		t.Fatalf("response/cache = %#v / %#v", response, cache.evicted)
	}
}

func TestReloadInvalidatesNamespacesWithoutScanning(t *testing.T) {
	cache := &cacheStub{}
	response, err := newTestService(cache).Reload(context.Background(), ReloadRequest{
		Namespaces: []string{NamespacePolicyConfig, NamespacePolicyTypes},
		UpdatedBy:  "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"policy-service:policy:config:generation",
		"policy-service:policy:types:generation",
	}
	if !reflect.DeepEqual(cache.incremented, want) || !reflect.DeepEqual(response.ReloadedNamespaces, []string{NamespacePolicyConfig, NamespacePolicyTypes}) {
		t.Fatalf("response/cache = %#v / %#v", response, cache.incremented)
	}
}

func TestReloadAllInvalidatesEveryNamespace(t *testing.T) {
	cache := &cacheStub{}
	response, err := newTestService(cache).Reload(context.Background(), ReloadRequest{
		Namespaces: []string{NamespaceAll},
		UpdatedBy:  "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		NamespaceUnits,
		NamespacePolicyTypes,
		NamespacePolicyGroups,
		NamespacePolicies,
		NamespaceCustomPolicies,
		NamespacePolicyConfig,
	}
	if !reflect.DeepEqual(response.ReloadedNamespaces, want) {
		t.Fatalf("namespaces = %#v, want %#v", response.ReloadedNamespaces, want)
	}
	if len(cache.incremented) != len(want) {
		t.Fatalf("incremented = %#v", cache.incremented)
	}
}

func TestReloadRejectsMixedOrUnknownTargets(t *testing.T) {
	service := newTestService(&cacheStub{})
	for _, request := range []ReloadRequest{
		{Key: "policy:key", Namespaces: []string{NamespacePolicies}, UpdatedBy: "admin"},
		{Namespaces: []string{"other-service"}, UpdatedBy: "admin"},
		{Namespaces: []string{NamespacePolicies}},
	} {
		if _, err := service.Reload(context.Background(), request); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("Reload(%#v) error = %v", request, err)
		}
	}
}
