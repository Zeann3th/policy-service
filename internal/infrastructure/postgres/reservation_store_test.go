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
	"github.com/zeann3th/policy-service/internal/core/unit"
)

// These tests write to a real, migrated database. They run only when
// TEST_DATABASE_URL is set, so they never touch the .env database.
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

func policyTypeID(t *testing.T, pool *pgxpool.Pool, code string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(context.Background(), "SELECT id FROM policy_types WHERE code = $1", code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// dailyLimit returns a DAILY TOTAL_MAX_AMT limit for both directions.
func dailyLimit(t *testing.T, pool *pgxpool.Pool, limit string) resolverStub {
	return resolverStub{{
		ID: 1, PolicyTypeID: policyTypeID(t, pool, "TOTAL_MAX_AMT"), PolicyType: "TOTAL_MAX_AMT",
		Kind: policy.KindLimit, Unit: unit.CodeAmount, TransactionType: policy.All, Channel: policy.All,
		Direction: policy.DirectionAll, WindowType: policy.WindowDaily, Operator: policy.OpLTE, Value: decimal.RequireFromString(limit),
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
