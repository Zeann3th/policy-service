package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/dryrun"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// ReservationStore implements reservation.Store on Postgres.
type ReservationStore struct {
	db      *pgxpool.Pool
	queries *codegen.Queries
}

func NewReservationStore(db *pgxpool.Pool) *ReservationStore {
	return &ReservationStore{db: db, queries: codegen.New(db)}
}

func (s *ReservationStore) Create(ctx context.Context, value reservation.Reservation) (reservation.Reservation, error) {
	createdAt, err := s.queries.CreateReservation(ctx, codegen.CreateReservationParams{
		ID:                 value.ID,
		RequestID:          value.RequestID,
		Status:             string(value.Status),
		TransactionType:    value.TransactionType,
		Channel:            value.Channel,
		Currency:           value.Currency,
		Amount:             value.Amount,
		SenderWalletID:     value.Sender.WalletID,
		SenderWalletType:   value.Sender.WalletType,
		SenderTier:         value.Sender.Tier,
		ReceiverWalletID:   value.Receiver.WalletID,
		ReceiverWalletType: value.Receiver.WalletType,
		ReceiverTier:       value.Receiver.Tier,
	})
	if err != nil {
		return reservation.Reservation{}, mapDBError(err)
	}
	value.CreatedAt = createdAt
	return value, nil
}

func (s *ReservationStore) Find(ctx context.Context, requestID string) (reservation.Reservation, error) {
	row, err := s.queries.GetReservationByRequestID(ctx, requestID)
	if err != nil {
		return reservation.Reservation{}, mapDBError(err)
	}
	return reservation.Reservation{
		ID:              row.ID,
		RequestID:       row.RequestID,
		Status:          reservation.Status(row.Status),
		TransactionType: row.TransactionType,
		Channel:         row.Channel,
		Currency:        row.Currency,
		Amount:          row.Amount,
		Sender:          reservation.Party{WalletID: row.SenderWalletID, WalletType: row.SenderWalletType, Tier: row.SenderTier},
		Receiver:        reservation.Party{WalletID: row.ReceiverWalletID, WalletType: row.ReceiverWalletType, Tier: row.ReceiverTier},
		Result:          row.Result,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func (s *ReservationStore) Fail(ctx context.Context, id uuid.UUID, createdAt time.Time) error {
	return finish(ctx, s.queries, id, createdAt, reservation.StatusFailed, nil)
}

// Transact locks the wallets in ascending lock-key order, so transactions
// over overlapping wallets always queue in the same order and cannot
// deadlock. Sorting the keys rather than the wallet ids keeps the order total
// even when two ids hash to the same key. The advisory locks end with the
// transaction.
func (s *ReservationStore) Transact(ctx context.Context, walletIDs []string, fn func(reservation.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := codegen.New(tx)
	for _, key := range lockKeys(walletIDs) {
		if err := q.Lock(ctx, key); err != nil {
			return fmt.Errorf("lock wallets %v: %w", walletIDs, err)
		}
	}
	if err := fn(&reservationTx{queries: q}); err != nil {
		return mapDBError(err)
	}
	return tx.Commit(ctx)
}

// lockKeys maps wallet ids to sorted, distinct advisory lock keys.
func lockKeys(walletIDs []string) []int64 {
	keys := make([]int64, len(walletIDs))
	for i, id := range walletIDs {
		hash := fnv.New64a()
		hash.Write([]byte(id))
		keys[i] = int64(hash.Sum64())
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

type reservationTx struct {
	queries *codegen.Queries
}

func (t *reservationTx) Usage(ctx context.Context, key reservation.UsageKey) (decimal.Decimal, error) {
	return usage(ctx, t.queries, key)
}

// Usage reads a usage window outside any transaction and takes no lock; the
// value may be stale by the time it is used (dry runs only).
func (s *ReservationStore) Usage(ctx context.Context, key reservation.UsageKey) (decimal.Decimal, error) {
	return usage(ctx, s.queries, key)
}

// usage is a window's total, zero when nothing was reserved in it yet.
func usage(ctx context.Context, q *codegen.Queries, key reservation.UsageKey) (decimal.Decimal, error) {
	value, err := q.GetUsageValue(ctx, codegen.GetUsageValueParams{
		WalletID:        key.WalletID,
		PolicyTypeID:    key.PolicyTypeID,
		TransactionType: key.TransactionType,
		Channel:         key.Channel,
		Direction:       string(key.Direction),
		WindowType:      string(key.WindowType),
		UnitID:          key.UnitID,
		WindowStart:     date(key.WindowStart),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return decimal.Zero, nil
	}
	return value, err
}

func (t *reservationTx) Apply(ctx context.Context, entry reservation.LedgerEntry) (decimal.Decimal, error) {
	valueAfter, err := t.queries.AddUsage(ctx, codegen.AddUsageParams{
		WalletID:        entry.WalletID,
		PolicyTypeID:    entry.PolicyTypeID,
		TransactionType: entry.TransactionType,
		Channel:         entry.Channel,
		Direction:       string(entry.Direction),
		WindowType:      string(entry.WindowType),
		UnitID:          entry.UnitID,
		WindowStart:     date(entry.WindowStart),
		Delta:           entry.Delta,
	})
	if err != nil {
		return decimal.Zero, fmt.Errorf("add usage: %w", err)
	}
	var operator *string
	if entry.Operator != nil {
		value := string(*entry.Operator)
		operator = &value
	}
	err = t.queries.CreateUsageLedger(ctx, codegen.CreateUsageLedgerParams{
		ReservationID:        entry.ReservationID,
		ReservationCreatedAt: entry.ReservationCreatedAt,
		EntryType:            string(entry.EntryType),
		WalletID:             entry.WalletID,
		PolicyTypeID:         entry.PolicyTypeID,
		TransactionType:      entry.TransactionType,
		Channel:              entry.Channel,
		Direction:            string(entry.Direction),
		WindowType:           string(entry.WindowType),
		UnitID:               entry.UnitID,
		WindowStart:          date(entry.WindowStart),
		Delta:                entry.Delta,
		PolicyID:             entry.PolicyID,
		PolicyTypeCode:       entry.PolicyTypeCode,
		Operator:             operator,
		LimitValue:           entry.LimitValue,
		ValueAfter:           valueAfter,
	})
	if err != nil {
		return decimal.Zero, fmt.Errorf("create usage ledger: %w", err)
	}
	return valueAfter, nil
}

func (t *reservationTx) Finish(ctx context.Context, id uuid.UUID, createdAt time.Time, status reservation.Status, result json.RawMessage) error {
	return finish(ctx, t.queries, id, createdAt, status, result)
}

func (t *reservationTx) Transition(ctx context.Context, id uuid.UUID, createdAt time.Time, from, to reservation.Status) (time.Time, error) {
	updatedAt, err := t.queries.TransitionReservation(ctx, codegen.TransitionReservationParams{
		Status:     string(to),
		ID:         id,
		CreatedAt:  createdAt,
		FromStatus: string(from),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, core.ErrConflict.WithMessage(fmt.Sprintf("reservation %s is no longer %s", id, from))
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("set reservation %s %s: %w", id, to, err)
	}
	if updatedAt == nil {
		return time.Time{}, fmt.Errorf("set reservation %s %s: updated_at not set", id, to)
	}
	return *updatedAt, nil
}

func (t *reservationTx) Entries(ctx context.Context, id uuid.UUID, createdAt time.Time, entryType reservation.EntryType) ([]reservation.LedgerEntry, error) {
	rows, err := t.queries.ListUsageLedger(ctx, codegen.ListUsageLedgerParams{
		ReservationID:        id,
		ReservationCreatedAt: createdAt,
		EntryType:            string(entryType),
	})
	if err != nil {
		return nil, fmt.Errorf("list usage ledger: %w", err)
	}
	entries := make([]reservation.LedgerEntry, len(rows))
	for i, row := range rows {
		var operator *policy.Operator
		if row.Operator != nil {
			value := policy.Operator(*row.Operator)
			operator = &value
		}
		entries[i] = reservation.LedgerEntry{
			ReservationID:        row.ReservationID,
			ReservationCreatedAt: row.ReservationCreatedAt,
			EntryType:            reservation.EntryType(row.EntryType),
			UsageKey: reservation.UsageKey{
				WalletID:        row.WalletID,
				PolicyTypeID:    row.PolicyTypeID,
				TransactionType: row.TransactionType,
				Channel:         row.Channel,
				Direction:       policy.Direction(row.Direction),
				WindowType:      policy.WindowType(row.WindowType),
				UnitID:          row.UnitID,
				WindowStart:     row.WindowStart.Time,
			},
			Delta:          row.Delta,
			PolicyID:       row.PolicyID,
			PolicyTypeCode: row.PolicyTypeCode,
			Operator:       operator,
			LimitValue:     row.LimitValue,
			ValueAfter:     row.ValueAfter,
			CreatedAt:      row.CreatedAt,
		}
	}
	return entries, nil
}

func finish(ctx context.Context, q *codegen.Queries, id uuid.UUID, createdAt time.Time, status reservation.Status, result json.RawMessage) error {
	rows, err := q.UpdateReservationStatus(ctx, codegen.UpdateReservationStatusParams{
		Status:     string(status),
		Result:     result,
		ID:         id,
		CreatedAt:  createdAt,
		FromStatus: string(reservation.StatusProcessing),
	})
	if err != nil {
		return fmt.Errorf("set reservation %s %s: %w", id, status, err)
	}
	if rows != 1 {
		return fmt.Errorf("set reservation %s %s: not PROCESSING", id, status)
	}
	return nil
}

func date(value time.Time) pgtype.Date {
	return pgtype.Date{Time: value, Valid: true}
}

var (
	_ reservation.Store  = (*ReservationStore)(nil)
	_ dryrun.UsageReader = (*ReservationStore)(nil)
)
