package limit

import (
	"context"
	"errors"
	"testing"
)

type fakeLimitStore struct {
	groupID      int64
	groupErr     error
	values       []LimitValue
	valuesErr    error
	groupQuery   GetLimitCommand
	valuesGroup  int64
	valuesCalled bool
}

func (s *fakeLimitStore) HasLimitTypes(context.Context) (bool, error) {
	return false, nil
}

func (s *fakeLimitStore) GetLimitGroupID(_ context.Context, walletTypeCode, currencyCode, tierCode string) (int64, error) {
	s.groupQuery = GetLimitCommand{
		WalletTypeCode: walletTypeCode,
		CurrencyCode:   currencyCode,
		TierCode:       tierCode,
	}
	return s.groupID, s.groupErr
}

func (s *fakeLimitStore) GetLimitValuesByGroupID(_ context.Context, groupID int64) ([]LimitValue, error) {
	s.valuesCalled = true
	s.valuesGroup = groupID
	return s.values, s.valuesErr
}

func TestGetLimitConfigReturnsGroupLimits(t *testing.T) {
	store := &fakeLimitStore{
		groupID: 42,
		values: []LimitValue{
			{LimitGroupID: 42, LimitTypeCode: "DAILY_AMOUNT"},
		},
	}
	service := NewService(nil, store)
	command := GetLimitCommand{
		WalletTypeCode: "PERSONAL",
		CurrencyCode:   "USD",
		TierCode:       "TIER_1",
		WalletID:       "wallet-123",
	}

	values, err := service.GetLimitConfig(context.Background(), command)
	if err != nil {
		t.Fatalf("GetLimitConfig() error = %v", err)
	}
	if len(values) != 1 || values[0].LimitGroupID != store.groupID {
		t.Fatalf("GetLimitConfig() values = %#v", values)
	}
	if store.groupQuery.WalletTypeCode != command.WalletTypeCode ||
		store.groupQuery.CurrencyCode != command.CurrencyCode ||
		store.groupQuery.TierCode != command.TierCode {
		t.Fatalf("GetLimitConfig() group query = %#v", store.groupQuery)
	}
	if store.valuesGroup != store.groupID {
		t.Fatalf("GetLimitConfig() values group ID = %d, want %d", store.valuesGroup, store.groupID)
	}
}

func TestGetLimitConfigStopsWhenGroupLookupFails(t *testing.T) {
	groupErr := errors.New("group lookup failed")
	store := &fakeLimitStore{groupErr: groupErr}
	service := NewService(nil, store)

	_, err := service.GetLimitConfig(context.Background(), GetLimitCommand{})
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
	service := NewService(nil, store)

	_, err := service.GetLimitConfig(context.Background(), GetLimitCommand{})
	if !errors.Is(err, valuesErr) {
		t.Fatalf("GetLimitConfig() error = %v, want %v", err, valuesErr)
	}
}
