package unit

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type storeStub struct {
	Store
	limit  int32
	offset int32
}

func (s *storeStub) List(context.Context, int32, int32) ([]Unit, int64, error) {
	return nil, 0, nil
}

type cacheStub struct{}

func (*cacheStub) Get(context.Context, string) (string, error) { return "0", nil }
func (*cacheStub) Set(context.Context, string, string) error   { return nil }
func (*cacheStub) SetWithTTL(context.Context, string, string, time.Duration) error {
	return nil
}
func (*cacheStub) TrySet(context.Context, string, string) (bool, error) { return false, nil }
func (*cacheStub) TrySetWithTTL(context.Context, string, string, time.Duration) (bool, error) {
	return false, nil
}
func (*cacheStub) Evict(context.Context, string) (bool, error) { return false, nil }
func (*cacheStub) EvictAll(context.Context) error              { return nil }
func (*cacheStub) Increment(context.Context, string) (int64, error) {
	return 1, nil
}

func TestListDefaultsToFirstTwenty(t *testing.T) {
	store := &storeStub{}
	service := NewService(store, cachecore.NewVersioned(slog.New(slog.NewTextHandler(io.Discard, nil)), &cacheStub{}, "policy-service"))
	response, err := service.List(context.Background(), core.PageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if response.Page != 1 || response.Limit != 20 {
		t.Fatalf("page = %#v", response)
	}
}
