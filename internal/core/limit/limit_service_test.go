package limit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

type fakeLimitStore struct {
	LimitStore
	units        []Unit
	unitsErr     error
	unitsCalled  bool
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

func (s *fakeLimitStore) GetUnits(context.Context) ([]Unit, error) {
	s.unitsCalled = true
	return s.units, s.unitsErr
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
	value         string
	getErr        error
	setErr        error
	setValues     map[string]string
	getKey        string
	generationErr error
}

func (c *fakeCache) Get(_ context.Context, key string) (string, error) {
	c.getKey = key
	if strings.HasSuffix(key, ":generation") && c.generationErr != nil {
		return "", c.generationErr
	}
	if strings.HasSuffix(key, ":generation") {
		return "0", nil
	}
	return c.value, c.getErr
}

func (c *fakeCache) Set(_ context.Context, key string, value string) error {
	if c.setValues == nil {
		c.setValues = make(map[string]string)
	}
	c.setValues[key] = value
	return c.setErr
}

func (c *fakeCache) SetWithTTL(_ context.Context, key, value string, _ time.Duration) error {
	return c.Set(context.Background(), key, value)
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

func (c *fakeCache) Increment(context.Context, string) (int64, error) { return 1, nil }

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
		units:   []Unit{{ID: 3, Code: "AMOUNT"}},
		values: []LimitValue{
			{
				LimitGroupID:    42,
				LimitTypeID:     7,
				LimitType:       "TXN_MAX_AMT",
				UnitID:          3,
				TransactionType: "TRANSFER",
				Channel:         "MOBILE",
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(100),
			},
		},
	}
	cache := &fakeCache{getErr: cacheport.ErrMiss}
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
		Unit:            "AMOUNT",
		Source:          SourceSystem,
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
		got.Unit != want.Unit ||
		got.Source != want.Source ||
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
	cachedValue, ok := cache.setValues["limit-service:limit:limit-config:0:PERSONAL:USD:TIER_1"]
	if !ok {
		t.Fatalf("GetLimitConfig() cache keys = %#v", cache.setValues)
	}
	cachedValues, err := core.Decode[[]LimitValue](cachedValue)
	if err != nil || len(*cachedValues) != 1 {
		t.Fatalf("GetLimitConfig() cached value = %q, error = %v", cachedValue, err)
	}
}

func TestGetLimitConfigReturnsCachedLimits(t *testing.T) {
	want := []LimitValue{{LimitGroupID: 42, LimitTypeID: 7, LimitType: "TXN_MAX_AMT", UnitID: 3}}
	encoded, err := core.Encode(&want)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	store := &fakeLimitStore{units: []Unit{{ID: 3, Code: "AMOUNT"}}}
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
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

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
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

	_, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{})
	if !errors.Is(err, valuesErr) {
		t.Fatalf("GetLimitConfig() error = %v, want %v", err, valuesErr)
	}
}

func TestGetLimitConfigOverridesExactSystemRuleWithUserLimit(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		units:   []Unit{{ID: 3, Code: "AMOUNT"}},
		values: []LimitValue{{
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			UnitID:          3,
			TransactionType: All,
			Channel:         All,
			Operator:        OpLTE,
			Value:           decimal.NewFromInt(2_000_000),
		}},
		userLimits: []UserLimit{{
			WalletID:        "wallet-123",
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			UnitID:          3,
			TransactionType: All,
			Channel:         All,
			Operator:        OpLTE,
			Value:           decimal.NewFromInt(100_000),
		}},
	}
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

	response, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{
		WalletType: "CUSTOMER",
		Currency:   "VND",
		Tier:       "BASIC",
		WalletID:   "wallet-123",
	})
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if len(response.Rules) != 1 ||
		response.Rules[0].Source != SourceUser ||
		!response.Rules[0].Value.Equal(decimal.NewFromInt(100_000)) {
		t.Fatalf("GetLimitConfig() response = %#v", response)
	}
	if store.userWalletID != "wallet-123" {
		t.Fatalf("GetActiveUserLimits() wallet ID = %q", store.userWalletID)
	}
}

func TestGetLimitConfigAppendsUserRuleWithDifferentScope(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		units: []Unit{
			{ID: 3, Code: "AMOUNT"},
		},
		values: []LimitValue{{
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			UnitID:          3,
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
				UnitID:          3,
				TransactionType: All,
				Channel:         "USSD",
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(100_000),
			},
			{
				WalletID:        "wallet-123",
				LimitTypeID:     7,
				LimitType:       "TXN_MAX_AMT",
				UnitID:          3,
				TransactionType: "TRANSFER_OUT",
				Channel:         All,
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(200_000),
			},
			{
				WalletID:        "wallet-123",
				LimitTypeID:     8,
				LimitType:       "DAILY_OUT_MAX",
				UnitID:          3,
				TransactionType: All,
				Channel:         All,
				Operator:        OpLTE,
				Value:           decimal.NewFromInt(5_000_000),
			},
		},
	}
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

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

func TestGetLimitConfigFiltersRequestedAndGlobalTransactionRules(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		units:   []Unit{{ID: 3, Code: "AMOUNT"}},
		values: []LimitValue{
			{LimitTypeID: 7, LimitType: "TXN_MAX_AMT", UnitID: 3, TransactionType: All, Channel: All},
			{LimitTypeID: 7, LimitType: "TXN_MAX_AMT", UnitID: 3, TransactionType: "TR0002", Channel: All},
		},
		userLimits: []UserLimit{
			{LimitTypeID: 7, LimitType: "TXN_MAX_AMT", UnitID: 3, TransactionType: "TR0001", Channel: "USSD"},
			{LimitTypeID: 7, LimitType: "TXN_MAX_AMT", UnitID: 3, TransactionType: "TR0002", Channel: "USSD"},
		},
	}
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

	response, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{
		WalletID:        "wallet-123",
		TransactionType: "TR0001",
	})
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if len(response.Rules) != 2 ||
		response.Rules[0].TransactionType != All ||
		response.Rules[0].Source != SourceSystem ||
		response.Rules[1].TransactionType != "TR0001" ||
		response.Rules[1].Source != SourceUser {
		t.Fatalf("GetLimitConfig() response = %#v", response)
	}
}

func TestGetLimitConfigReturnsEmptyRulesWhenTransactionDoesNotMatch(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		values: []LimitValue{{
			LimitTypeID:     7,
			LimitType:       "TXN_MAX_AMT",
			UnitID:          3,
			TransactionType: "TR0002",
		}},
	}
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

	response, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{TransactionType: "TR0001"})
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if response.Rules == nil || len(response.Rules) != 0 {
		t.Fatalf("GetLimitConfig() response = %#v, want empty rules", response)
	}
	if store.unitsCalled {
		t.Fatal("GetLimitConfig() queried units for an empty result")
	}
}

func TestGetLimitConfigReturnsErrorForMissingUnit(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		values: []LimitValue{{
			LimitTypeID: 7,
			LimitType:   "TXN_MAX_AMT",
			UnitID:      99,
		}},
	}
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

	_, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{})
	if err == nil {
		t.Fatal("GetLimitConfig() error = nil, want missing unit error")
	}
}

func TestGetLimitConfigReturnsUserLimitError(t *testing.T) {
	userErr := errors.New("user limits lookup failed")
	store := &fakeLimitStore{groupID: 42, userErr: userErr}
	service := newTestService(store, &fakeCache{getErr: cacheport.ErrMiss})

	_, err := service.GetLimitConfig(context.Background(), GetLimitConfigRequest{WalletID: "wallet-123"})
	if !errors.Is(err, userErr) {
		t.Fatalf("GetLimitConfig() error = %v, want %v", err, userErr)
	}
}

type limitStoreStub struct {
	LimitStore
	listLimit, listOffset int32
	createValue           CreateLimitValueRequest
}

func (s *limitStoreStub) ListUnits(_ context.Context, limit, offset int32) ([]Unit, int64, error) {
	s.listLimit, s.listOffset = limit, offset
	return []Unit{{ID: 1, Code: "AMOUNT", Active: true}}, 1, nil
}

func (s *limitStoreStub) CreateLimitValue(_ context.Context, _ int64, request CreateLimitValueRequest) (LimitValue, error) {
	s.createValue = request
	return LimitValue{ID: 1, TransactionType: request.TransactionType, Channel: request.Channel}, nil
}
func (s *limitStoreStub) GetLimitGroup(context.Context, int64) (LimitGroup, error) {
	return LimitGroup{}, nil
}

func TestListUnitsDefaultsToFirstTwenty(t *testing.T) {
	store := &limitStoreStub{}
	service := newTestService(store, &fakeCache{getErr: context.Canceled})

	page, err := service.ListUnits(context.Background(), PageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.Limit != 20 || page.Total != 1 || store.listLimit != 20 || store.listOffset != 0 {
		t.Fatalf("page = %#v; store limit/offset = %d/%d", page, store.listLimit, store.listOffset)
	}
}

func TestCreateLimitValueDefaultsScopeToAll(t *testing.T) {
	store := &limitStoreStub{}
	service := newTestService(store, &fakeCache{getErr: context.Canceled})

	value := decimal.Zero
	_, err := service.CreateLimitValue(context.Background(), 7, CreateLimitValueRequest{Value: &value, UpdatedBy: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if store.createValue.TransactionType != All || store.createValue.Channel != All {
		t.Fatalf("request = %#v", store.createValue)
	}
}
