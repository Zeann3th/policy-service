package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
)

// These tests write to a real, migrated database. They run only when
// TEST_DATABASE_URL is set, so they never touch the local development database.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

type resolverStub []policy.Policy

func (r resolverStub) Resolve(_ context.Context, request policy.ResolveRequest) ([]policy.Policy, error) {
	matched := make([]policy.Policy, 0, len(r))
	for _, value := range r {
		if value.Matches(request.TransactionType, request.Channel, request.Direction) {
			matched = append(matched, value)
		}
	}
	return matched, nil
}

func unitID(t *testing.T, pool *pgxpool.Pool, code string) *int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), "SELECT id FROM units WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return &id
}

func policyTypeID(t *testing.T, pool *pgxpool.Pool, code string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), "SELECT id FROM policy_types WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// dailyLimit returns a DAILY TOTAL_MAX limit for both directions.
func dailyLimit(t *testing.T, pool *pgxpool.Pool, limit string) resolverStub {
	return resolverStub{{
		ID: 1, PolicyTypeID: policyTypeID(t, pool, "TOTAL_MAX"), PolicyType: "TOTAL_MAX",
		Kind: policy.KindLimit, UnitID: unitID(t, pool, policy.UnitAmount), Unit: policy.UnitAmount, TransactionType: policy.All, Channel: policy.All,
		Direction: policy.DirectionAll, WindowType: policy.WindowDaily, Operator: policy.OpLTE, Value: decimal.RequireFromString(limit), IsEnforced: true,
	}}
}

func reserveRequest(sender, receiver, amount string) reservation.ReserveRequest {
	value := decimal.RequireFromString(amount)
	return reservation.ReserveRequest{
		RequestID: uuid.NewString(), TransactionType: "TRANSFER", Currency: "VND", Amount: &value,
		Sender:   reservation.PartyRequest{WalletID: sender, WalletType: "PERSONAL", Tier: "T1"},
		Receiver: reservation.PartyRequest{WalletID: receiver, WalletType: "PERSONAL", Tier: "T1"},
	}
}

func wallet(name string) string {
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
}

func usageValue(t *testing.T, pool *pgxpool.Pool, walletID string) decimal.Decimal {
	t.Helper()
	var value decimal.Decimal
	err := pool.QueryRow(context.Background(), "SELECT COALESCE(sum(value), 0) FROM usages WHERE wallet_id = $1", walletID).Scan(&value)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func ledgerSum(t *testing.T, pool *pgxpool.Pool, walletID string) decimal.Decimal {
	t.Helper()
	var value decimal.Decimal
	err := pool.QueryRow(context.Background(), "SELECT COALESCE(sum(delta), 0) FROM usage_ledger WHERE wallet_id = $1", walletID).Scan(&value)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestReserveConcurrentRequestsNeverExceedLimit(t *testing.T) {
	pool := testPool(t)
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000"))
	sender, receiver := wallet("sender"), wallet("receiver")

	const requests = 20 // 20 x 100 against a daily limit of 1000
	statuses := make(chan reservation.Status, requests)
	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() {
			response, err := service.Reserve(context.Background(), reserveRequest(sender, receiver, "100"))
			if err != nil {
				t.Error(err)
				return
			}
			statuses <- response.Status
		})
	}
	wg.Wait()
	close(statuses)

	counts := map[reservation.Status]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[reservation.StatusReserved] != 10 || counts[reservation.StatusRejected] != 10 {
		t.Fatalf("statuses = %v, want 10 RESERVED and 10 REJECTED", counts)
	}
	for _, walletID := range []string{sender, receiver} {
		if usage, ledger := usageValue(t, pool, walletID), ledgerSum(t, pool, walletID); !usage.Equal(decimal.NewFromInt(1000)) || !ledger.Equal(usage) {
			t.Errorf("%s usage = %s, ledger = %s, want 1000", walletID, usage, ledger)
		}
	}
}

func TestReserveOpposingTransfersDoNotDeadlock(t *testing.T) {
	pool := testPool(t)
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000000"))
	a, b := wallet("a"), wallet("b")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Go(func() {
			sender, receiver := a, b
			if i%2 == 1 {
				sender, receiver = b, a
			}
			if _, err := service.Reserve(ctx, reserveRequest(sender, receiver, "1")); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	// each wallet sent 20 and received 20; the limit covers both directions
	for _, walletID := range []string{a, b} {
		if usage := usageValue(t, pool, walletID); !usage.Equal(decimal.NewFromInt(40)) {
			t.Errorf("%s usage = %s, want 40", walletID, usage)
		}
	}
}

func TestReserveRejectsDuplicateRequestID(t *testing.T) {
	pool := testPool(t)
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000"))
	request := reserveRequest(wallet("sender"), wallet("receiver"), "1")

	if _, err := service.Reserve(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reserve(context.Background(), request); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("duplicate err = %v, want conflict", err)
	}
}

func TestReserveFailureRollsBackAndMarksFailed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	limit := dailyLimit(t, pool, "1000")
	// the same scope twice on one side: the second ledger insert violates
	// the primary key after the first usage upsert has run
	limit = append(limit, limit[0])
	service := reservation.NewService(NewReservationStore(pool), limit)
	request := reserveRequest(wallet("sender"), wallet("receiver"), "100")

	if _, err := service.Reserve(ctx, request); err == nil {
		t.Fatal("Reserve() succeeded, want ledger conflict")
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT status FROM reservations WHERE request_id = $1", request.RequestID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(reservation.StatusFailed) {
		t.Fatalf("status = %s, want FAILED", status)
	}
	if usage := usageValue(t, pool, request.Sender.WalletID); !usage.IsZero() {
		t.Fatalf("usage = %s, want the upsert rolled back", usage)
	}
	if ledger := ledgerSum(t, pool, request.Sender.WalletID); !ledger.IsZero() {
		t.Fatalf("ledger = %s, want no entries", ledger)
	}
}

func TestLedgerLivesInTheReservationMonthPartition(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000"))
	response, err := service.Reserve(ctx, reserveRequest(wallet("sender"), wallet("receiver"), "1"))
	if err != nil {
		t.Fatal(err)
	}
	var reservationPartition, ledgerPartition string
	err = pool.QueryRow(ctx, `
		SELECT r.tableoid::regclass::text, l.tableoid::regclass::text
		FROM reservations r JOIN usage_ledger l ON l.reservation_id = r.id AND l.reservation_created_at = r.created_at
		WHERE r.id = $1 LIMIT 1`, response.ReservationID).Scan(&reservationPartition, &ledgerPartition)
	if err != nil {
		t.Fatal(err)
	}
	month := time.Now().UTC().Format("p200601")
	if reservationPartition != "reservations_"+month+"01" || ledgerPartition != "usage_ledger_"+month+"01" {
		t.Fatalf("partitions = %s, %s", reservationPartition, ledgerPartition)
	}
}

func TestLockKeysAreSortedAndDistinct(t *testing.T) {
	keys := lockKeys([]string{"b", "a", "b"})
	if len(keys) != 2 || keys[0] >= keys[1] {
		t.Fatalf("keys = %v", keys)
	}
	if reversed := lockKeys([]string{"a", "b"}); reversed[0] != keys[0] || reversed[1] != keys[1] {
		t.Fatalf("order depends on input: %v vs %v", reversed, keys)
	}
}

func TestReleaseRestoresUsageAndRecordsReversal(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000"))
	request := reserveRequest(wallet("sender"), wallet("receiver"), "600")

	if _, err := service.Reserve(ctx, request); err != nil {
		t.Fatal(err)
	}
	response, err := service.Release(ctx, request.RequestID)
	if err != nil || response.Status != reservation.StatusReleased {
		t.Fatalf("Release() = %+v, %v", response, err)
	}
	for _, walletID := range []string{request.Sender.WalletID, request.Receiver.WalletID} {
		if usage, ledger := usageValue(t, pool, walletID), ledgerSum(t, pool, walletID); !usage.IsZero() || !ledger.IsZero() {
			t.Errorf("%s usage = %s, ledger = %s, want 0", walletID, usage, ledger)
		}
	}
	var releases int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM usage_ledger WHERE reservation_id = $1 AND entry_type = $2",
		response.ReservationID, reservation.EntryRelease).Scan(&releases); err != nil || releases != 2 {
		t.Fatalf("release entries = %d, %v", releases, err)
	}
	// the released 600 is available again
	if again, err := service.Reserve(ctx, reserveRequest(request.Sender.WalletID, request.Receiver.WalletID, "1000")); err != nil || again.Status != reservation.StatusReserved {
		t.Fatalf("Reserve() after release = %s, %v", again.Status, err)
	}
	if _, err := service.Release(ctx, "missing-"+request.RequestID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown request err = %v", err)
	}
}

func TestConcurrentReleaseReversesOnceAndAnswersAlike(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000"))
	request := reserveRequest(wallet("sender"), wallet("receiver"), "100")
	if _, err := service.Reserve(ctx, request); err != nil {
		t.Fatal(err)
	}

	const releases = 10
	responses := make(chan reservation.ReleaseResponse, releases)
	var wg sync.WaitGroup
	for range releases {
		wg.Go(func() {
			response, err := service.Release(ctx, request.RequestID)
			if err != nil {
				t.Error(err)
				return
			}
			responses <- response
		})
	}
	wg.Wait()
	close(responses)
	// every racer gets the winner's response
	var first *reservation.ReleaseResponse
	for response := range responses {
		if first == nil {
			first = &response
			continue
		}
		if response != *first {
			t.Fatalf("responses differ: %+v vs %+v", response, *first)
		}
	}
	if first == nil || first.Status != reservation.StatusReleased || first.ResponseTime.IsZero() {
		t.Fatalf("first = %+v", first)
	}
	var releaseEntries int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM usage_ledger WHERE reservation_id = $1 AND entry_type = $2",
		first.ReservationID, reservation.EntryRelease).Scan(&releaseEntries); err != nil || releaseEntries != 2 {
		t.Fatalf("release entries = %d, %v: reversed more than once", releaseEntries, err)
	}
	if usage := usageValue(t, pool, request.Sender.WalletID); !usage.IsZero() {
		t.Fatalf("usage = %s, want 0", usage)
	}
}

func TestReleaseRacingReservesKeepsUsageConsistent(t *testing.T) {
	pool := testPool(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	service := reservation.NewService(NewReservationStore(pool), dailyLimit(t, pool, "1000000"))
	sender, receiver := wallet("sender"), wallet("receiver")

	var requests []reservation.ReserveRequest
	for range 20 {
		request := reserveRequest(sender, receiver, "10")
		if _, err := service.Reserve(ctx, request); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
	}
	var wg sync.WaitGroup
	for i := range 40 {
		wg.Go(func() {
			var err error
			if i%2 == 0 {
				_, err = service.Release(ctx, requests[i/2].RequestID)
			} else {
				// opposite direction, so the wallet pair is locked in both orders
				_, err = service.Reserve(ctx, reserveRequest(receiver, sender, "1"))
			}
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	// 20 x 10 reserved then released, 20 x 1 reserved the other way
	for _, walletID := range []string{sender, receiver} {
		if usage, ledger := usageValue(t, pool, walletID), ledgerSum(t, pool, walletID); !usage.Equal(decimal.NewFromInt(20)) || !ledger.Equal(usage) {
			t.Errorf("%s usage = %s, ledger = %s, want 20", walletID, usage, ledger)
		}
	}
}
