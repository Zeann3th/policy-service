package limit

import (
	"context"
	"github.com/shopspring/decimal"
	"testing"
)

type adminStoreStub struct {
	LimitStore
	LimitAdminStore
	listLimit, listOffset int32
	createValue           CreateLimitValueRequest
}

func (s *adminStoreStub) ListUnits(_ context.Context, limit, offset int32) ([]Unit, int64, error) {
	s.listLimit, s.listOffset = limit, offset
	return []Unit{{ID: 1, Code: "AMOUNT", Active: true}}, 1, nil
}

func (s *adminStoreStub) CreateLimitValue(_ context.Context, _ int64, request CreateLimitValueRequest) (LimitValue, error) {
	s.createValue = request
	return LimitValue{ID: 1, TransactionType: request.TransactionType, Channel: request.Channel}, nil
}
func (s *adminStoreStub) GetLimitGroup(context.Context, int64) (LimitGroup, error) {
	return LimitGroup{}, nil
}

func TestListUnitsDefaultsToFirstTwenty(t *testing.T) {
	store := &adminStoreStub{}
	service := newTestService(store, &fakeCache{getErr: context.Canceled}).(LimitAdminService)

	page, err := service.ListUnits(context.Background(), PageRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.Limit != 20 || page.Total != 1 || store.listLimit != 20 || store.listOffset != 0 {
		t.Fatalf("page = %#v; store limit/offset = %d/%d", page, store.listLimit, store.listOffset)
	}
}

func TestCreateLimitValueDefaultsScopeToAll(t *testing.T) {
	store := &adminStoreStub{}
	service := newTestService(store, &fakeCache{getErr: context.Canceled}).(LimitAdminService)

	value := decimal.Zero
	_, err := service.CreateLimitValue(context.Background(), 7, CreateLimitValueRequest{Value: &value, UpdatedBy: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if store.createValue.TransactionType != All || store.createValue.Channel != All {
		t.Fatalf("request = %#v", store.createValue)
	}
}
