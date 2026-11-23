package reservation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

type resolverStub map[policy.Direction][]policy.Policy

func (r resolverStub) Resolve(_ context.Context, request policy.ResolveRequest) ([]policy.Policy, error) {
	return r[request.Direction], nil
}

type storeStub struct {
	created  bool
	locked   []string
	applied  []LedgerEntry
	status   Status
	failed   bool
	applyErr error
}

func (s *storeStub) Create(_ context.Context, value Reservation) (Reservation, error) {
	s.created = true
	value.CreatedAt = at
	return value, nil
}

func (s *storeStub) Fail(context.Context, uuid.UUID, time.Time) error {
	s.failed = true
	return nil
}

func (s *storeStub) Transact(_ context.Context, walletIDs []string, fn func(Tx) error) error {
	s.locked = walletIDs
	return fn(s)
}

func (s *storeStub) Usage(context.Context, UsageKey) (decimal.Decimal, error) {
	return decimal.Zero, nil
}

func (s *storeStub) Apply(_ context.Context, entry LedgerEntry) (decimal.Decimal, error) {
	if s.applyErr != nil {
		return decimal.Zero, s.applyErr
	}
	s.applied = append(s.applied, entry)
	return entry.Delta, nil
}

func (s *storeStub) Finish(_ context.Context, _ uuid.UUID, _ time.Time, status Status, result json.RawMessage) error {
	s.status = status
	return nil
}

func reserveRequest(amount string) ReserveRequest {
	value := d(amount)
	return ReserveRequest{
		RequestID: "req-1", TransactionType: "TRANSFER", Currency: "VND", Amount: &value,
		Sender:   PartyRequest{WalletID: "s", WalletType: "PERSONAL", Tier: "T1"},
		Receiver: PartyRequest{WalletID: "r", WalletType: "PERSONAL", Tier: "T1"},
	}
}

func dailyLimit(direction policy.Direction) policy.Policy {
	value := p(1, policy.KindLimit, unit.CodeAmount, policy.WindowDaily, policy.OpLTE, "1000")
	value.Direction = direction
	return value
}

func policies() resolverStub {
	return resolverStub{policy.DirectionOut: {dailyLimit(policy.DirectionOut)}, policy.DirectionIn: {dailyLimit(policy.DirectionIn)}}
}

func TestReserveAddsUsageForBothSides(t *testing.T) {
	store := &storeStub{}
	response, err := NewService(store, policies()).Reserve(context.Background(), reserveRequest("400"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != StatusReserved || store.status != StatusReserved || len(store.applied) != 2 {
		t.Fatalf("status = %s/%s, applied = %d", response.Status, store.status, len(store.applied))
	}
	if store.applied[0].WalletID != "s" || store.applied[1].WalletID != "r" || store.applied[0].ReservationCreatedAt != at {
		t.Fatalf("entries = %#v", store.applied)
	}
	if len(store.locked) != 2 {
		t.Fatalf("locked = %v", store.locked)
	}
}

func TestReserveRejectsWithoutAddingUsage(t *testing.T) {
	store := &storeStub{}
	response, err := NewService(store, policies()).Reserve(context.Background(), reserveRequest("1500"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != StatusRejected || store.status != StatusRejected || len(store.applied) != 0 || !response.Sender.IsViolated {
		t.Fatalf("response = %#v, applied = %d", response, len(store.applied))
	}
}

func TestReserveRequiresPoliciesOnBothSides(t *testing.T) {
	store := &storeStub{}
	_, err := NewService(store, resolverStub{policy.DirectionOut: {dailyLimit(policy.DirectionOut)}}).Reserve(context.Background(), reserveRequest("1"))
	if !errors.Is(err, core.ErrBadRequest) || store.created {
		t.Fatalf("err = %v, created = %v", err, store.created)
	}
}

func TestReserveMarksFailedWhenTransactionFails(t *testing.T) {
	store := &storeStub{applyErr: errors.New("db down")}
	_, err := NewService(store, policies()).Reserve(context.Background(), reserveRequest("400"))
	if err == nil || !store.failed {
		t.Fatalf("err = %v, failed = %v", err, store.failed)
	}
}

func TestReserveValidatesRequest(t *testing.T) {
	for name, request := range map[string]ReserveRequest{
		"zero amount":  reserveRequest("0"),
		"too precise":  reserveRequest("1.00001"),
		"same wallets": func() ReserveRequest { r := reserveRequest("1"); r.Receiver.WalletID = "s"; return r }(),
	} {
		if _, err := NewService(&storeStub{}, policies()).Reserve(context.Background(), request); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
