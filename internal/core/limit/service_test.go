package limit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
)

type fakeLimitStore struct {
	groupID      int64
	groupErr     error
	values       []LimitValue
	valuesErr    error
	groupQuery   GetLimitConfigRequest
	valuesGroup  int64
	valuesCalled bool
	groupCalled  bool
	userLimits   []UserLimit
	userErr      error
	userWalletID string
	userCalled   bool
}

func (s *fakeLimitStore) GetLimitGroupID(_ context.Context, walletType, currency, tier string) (int64, error) {
	s.groupCalled = true
	s.groupQuery = GetLimitConfigRequest{
		WalletType: walletType,
		Currency:   currency,
		Tier:       tier,
	}
	return s.groupID, s.groupErr
}

type fakeCache struct {
	value    string
	getErr   error
	setErr   error
	setKey   string
	setValue string
}

func (c *fakeCache) Get(context.Context, string) (string, error) {
	return c.value, c.getErr
}

func (c *fakeCache) Set(_ context.Context, key string, value string) error {
	c.setKey = key
	c.setValue = value
	return c.setErr
}

func (c *fakeCache) SetWithTTL(context.Context, string, string, time.Duration) error {
	return nil
}

func (c *fakeCache) TrySet(context.Context, string, string) (bool, error) {
	return false, nil
}

func (c *fakeCache) TrySetWithTTL(context.Context, string, string, time.Duration) (bool, error) {
	return false, nil
}

func (c *fakeCache) Evict(context.Context, string) (bool, error) {
	return false, nil
}

func (c *fakeCache) EvictAll(context.Context) error {
	return nil
}

func newTestService(store LimitStore, cache *fakeCache) LimitService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(logger, store, cache, "limit-service")
}

func (s *fakeLimitStore) GetLimitValuesByGroupID(_ context.Context, groupID int64) ([]LimitValue, error) {
	s.valuesCalled = true
	s.valuesGroup = groupID
	return s.values, s.valuesErr
}

func (s *fakeLimitStore) GetActiveUserLimits(_ context.Context, walletID string) ([]UserLimit, error) {
	s.userCalled = true
	s.userWalletID = walletID
	return s.userLimits, s.userErr
}

func TestGetLimitConfigReturnsGroupLimits(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		values: []LimitValue{
			{
				LimitGroupID:    42,
				LimitTypeID:     7,
				LimitType:       "TXN_MAX_AMT",
				TransactionType: "TRANSFER",
				Channel:         "MOBILE",
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(100),
			},
		},
	}
	cache := &fakeCache{getErr: errors.New("cache miss")}
	service := newTestService(store, cache)
	command := GetLimitConfigRequest{
		WalletType: "PERSONAL",
		Currency:   "USD",
		Tier:       "TIER_1",
		WalletID:   "wallet-123",
	}

	response, err := service.GetLimitConfig(context.Background(), command)
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	want := LimitRule{
		LimitType:       "TXN_MAX_AMT",
		TransactionType: "TRANSFER",
		Channel:         "MOBILE",
		Operator:        OpLTE,
		Value:           decimal.NewFromInt(100),
	}
	if len(response.Rules) != 1 {
		t.Fatalf("GetLimitConfig() response = %#v, want one rule", response)
	}
	got := response.Rules[0]
	if got.LimitType != want.LimitType ||
		got.TransactionType != want.TransactionType ||
		got.Channel != want.Channel ||
		got.Operator != want.Operator ||
		!got.Value.Equal(want.Value) {
		t.Fatalf("GetLimitConfig() response = %#v, want %#v", response, want)
	}
	if store.groupQuery.WalletType != command.WalletType ||
		store.groupQuery.Currency != command.Currency ||
		store.groupQuery.Tier != command.Tier {
		t.Fatalf("GetLimitConfig() group query = %#v", store.groupQuery)
	}
	if store.valuesGroup != store.groupID {
		t.Fatalf("GetLimitConfig() values group ID = %d, want %d", store.valuesGroup, store.groupID)
	}
	if cache.setKey != "limit-service:PERSONAL:USD:TIER_1" {
		t.Fatalf("GetLimitConfig() cache key = %q", cache.setKey)
	}
	cachedValues, err := core.Decode[[]LimitValue](cache.setValue)
	if err != nil || len(*cachedValues) != 1 {
		t.Fatalf("GetLimitConfig() cached value = %q, error = %v", cache.setValue, err)
	}
}

func TestGetLimitConfigReturnsCachedLimits(t *testing.T) {
	want := []LimitValue{{LimitGroupID: 42, LimitTypeID: 7, LimitType: "TXN_MAX_AMT"}}
	encoded, err := core.Encode(&want)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	store := &fakeLimitStore{}
	service := newTestService(store, &fakeCache{value: encoded})

	got, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{
		WalletType: "PERSONAL",
		Currency:   "USD",
		Tier:       "TIER_1",
	})
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if len(got.Rules) != 1 || got.Rules[0].LimitType != want[0].LimitType {
		t.Fatalf("GetLimitConfig() = %#v, want %#v", got, want)
	}
	if store.groupCalled {
		t.Fatal("GetLimitConfig() queried the store on a cache hit")
	}
}

func TestGetLimitConfigStopsWhenGroupLookupFails(t *testing.T) {
	groupErr := errors.New("group lookup failed")
	store := &fakeLimitStore{groupErr: groupErr}
	service := newTestService(store, &fakeCache{getErr: errors.New("cache miss")})

	_, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{})
	if !errors.Is(err, groupErr) {
		t.Fatalf("GetLimitConfig() error = %v, want %v", err, groupErr)
	}
	if store.valuesCalled {
		t.Fatalf("GetLimitConfig() queried values for group %d", store.valuesGroup)
	}
}

func TestGetLimitConfigReturnsLimitValuesError(t *testing.T) {
	valuesErr := errors.New("values lookup failed")
	store := &fakeLimitStore{groupID: 42, valuesErr: valuesErr}
	service := newTestService(store, &fakeCache{getErr: errors.New("cache miss")})

	_, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{})
	if !errors.Is(err, valuesErr) {
		t.Fatalf("GetLimitConfig() error = %v, want %v", err, valuesErr)
	}
}

func TestGetLimitConfigOverridesExactSystemRuleWithUserLimit(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		values: []LimitValue{{
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			TransactionType: All,
			Channel:         All,
			Operator:        OpLTE,
			Value:           decimal.NewFromInt(2_000_000),
		}},
		userLimits: []UserLimit{{
			WalletID:        "wallet-123",
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			TransactionType: All,
			Channel:         All,
			Operator:        OpLTE,
			Value:           decimal.NewFromInt(100_000),
		}},
	}
	service := newTestService(store, &fakeCache{getErr: errors.New("cache miss")})

	response, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{
		WalletType: "CUSTOMER",
		Currency:   "VND",
		Tier:       "BASIC",
		WalletID:   "wallet-123",
	})
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if len(response.Rules) != 1 || !response.Rules[0].Value.Equal(decimal.NewFromInt(100_000)) {
		t.Fatalf("GetLimitConfig() response = %#v", response)
	}
	if store.userWalletID != "wallet-123" {
		t.Fatalf("GetActiveUserLimits() wallet ID = %q", store.userWalletID)
	}
}

func TestGetLimitConfigAppendsUserRuleWithDifferentScope(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		values: []LimitValue{{
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			TransactionType: All,
			Channel:         All,
			Operator:        OpLTE,
			Value:           decimal.NewFromInt(2_000_000),
		}},
		userLimits: []UserLimit{
			{
				WalletID:        "wallet-123",
				LimitTypeID:     7,
				LimitType:       "TXN_MAX_AMT",
				TransactionType: All,
				Channel:         "USSD",
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(100_000),
			},
			{
				WalletID:        "wallet-123",
				LimitTypeID:     7,
				LimitType:       "TXN_MAX_AMT",
				TransactionType: "TRANSFER_OUT",
				Channel:         All,
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(200_000),
			},
			{
				WalletID:        "wallet-123",
				LimitTypeID:     8,
				LimitType:       "DAILY_OUT_MAX",
				TransactionType: All,
				Channel:         All,
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(5_000_000),
			},
		},
	}
	service := newTestService(store, &fakeCache{getErr: errors.New("cache miss")})

	response, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{WalletID: "wallet-123"})
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if len(response.Rules) != 4 ||
		response.Rules[1].TransactionType != All || response.Rules[1].Channel != "USSD" ||
		response.Rules[2].TransactionType != "TRANSFER_OUT" || response.Rules[2].Channel != All ||
		response.Rules[3].LimitType != "DAILY_OUT_MAX" {
		t.Fatalf("GetLimitConfig() response = %#v", response)
	}
}

func TestGetLimitConfigReturnsUserLimitError(t *testing.T) {
	userErr := errors.New("user limits lookup failed")
	store := &fakeLimitStore{groupID: 42, userErr: userErr}
	service := newTestService(store, &fakeCache{getErr: errors.New("cache miss")})

	_, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{WalletID: "wallet-123"})
	if !errors.Is(err, userErr) {
		t.Fatalf("GetLimitConfig() error = %v, want %v", err, userErr)
	}
}
