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
	found    Reservation
	entries  []LedgerEntry
	moved    []Status

	transitionErr error
}

var releasedAt = at.Add(time.Minute)

func (s *storeStub) Find(_ context.Context, requestID string) (Reservation, error) {
	if s.found.RequestID != requestID {
		return Reservation{}, core.ErrNotFound
	}
	return s.found, nil
}

func (s *storeStub) Transition(_ context.Context, _ uuid.UUID, _ time.Time, from, to Status) (time.Time, error) {
	if s.transitionErr != nil {
		return time.Time{}, s.transitionErr
	}
	s.moved = append(s.moved, from, to)
	return releasedAt, nil
}

func (s *storeStub) Entries(_ context.Context, _ uuid.UUID, _ time.Time, entryType EntryType) ([]LedgerEntry, error) {
	if entryType != EntryReserve {
		return nil, nil
	}
	return s.entries, nil
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
	value := p(1, policy.KindLimit, policy.UnitAmount, policy.WindowDaily, policy.OpLTE, "1000")
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

func reserved(status Status) *storeStub {
	key := UsageKey{WalletID: "s", PolicyTypeID: 1, TransactionType: policy.All, Channel: policy.All, Direction: policy.DirectionOut, WindowType: policy.WindowDaily, WindowStart: at}
	return &storeStub{
		found:   Reservation{RequestID: "req-1", Status: status, Sender: Party{WalletID: "s"}, Receiver: Party{WalletID: "r"}, CreatedAt: at},
		entries: []LedgerEntry{{EntryType: EntryReserve, UsageKey: key, Delta: d("400"), ReservationCreatedAt: at}},
	}
}

func TestReleaseReversesReserveEntries(t *testing.T) {
	store := reserved(StatusReserved)
	response, err := NewService(store, policies()).Release(context.Background(), "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != StatusReleased || len(store.moved) != 2 || store.moved[0] != StatusReserved || store.moved[1] != StatusReleased {
		t.Fatalf("status = %s, moved = %v", response.Status, store.moved)
	}
	if !response.ResponseTime.Equal(releasedAt) {
		t.Fatalf("responseTime = %s, want the stored release time %s", response.ResponseTime, releasedAt)
	}
	if len(store.applied) != 1 || store.applied[0].EntryType != EntryRelease || !store.applied[0].Delta.Equal(d("-400")) ||
		store.applied[0].UsageKey != store.entries[0].UsageKey || len(store.locked) != 2 {
		t.Fatalf("applied = %#v, locked = %v", store.applied, store.locked)
	}
}

func TestReleaseOnlyReserved(t *testing.T) {
	for _, status := range []Status{StatusProcessing, StatusRejected, StatusFailed} {
		store := reserved(status)
		if _, err := NewService(store, policies()).Release(context.Background(), "req-1"); !errors.Is(err, core.ErrConflict) || store.locked != nil {
			t.Errorf("%s: err = %v, locked = %v", status, err, store.locked)
		}
	}
	if _, err := NewService(reserved(StatusReserved), policies()).Release(context.Background(), "other"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("unknown request: err = %v", err)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	store := reserved(StatusReleased)
	store.found.UpdatedAt = &releasedAt
	response, err := NewService(store, policies()).Release(context.Background(), "req-1")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != StatusReleased || !response.ResponseTime.Equal(releasedAt) || store.locked != nil || len(store.applied) != 0 {
		t.Fatalf("response = %+v, locked = %v, applied = %d", response, store.locked, len(store.applied))
	}
}

func TestReleaseLosingARaceReturnsTheWinnersResponse(t *testing.T) {
	store := reserved(StatusReserved)
	store.transitionErr = core.ErrConflict
	winner := store.found
	winner.Status, winner.UpdatedAt = StatusReleased, &releasedAt
	service := NewService(&racedStore{storeStub: store, after: winner}, policies())

	response, err := service.Release(context.Background(), "req-1")
	if err != nil || response.Status != StatusReleased || !response.ResponseTime.Equal(releasedAt) {
		t.Fatalf("response = %+v, err = %v", response, err)
	}
}

// racedStore reports the reservation RESERVED on the first Find and as the
// concurrent winner left it afterwards.
type racedStore struct {
	*storeStub
	after Reservation
	finds int
}

func (s *racedStore) Find(ctx context.Context, requestID string) (Reservation, error) {
	s.finds++
	if s.finds > 1 {
		return s.after, nil
	}
	return s.storeStub.Find(ctx, requestID)
}
