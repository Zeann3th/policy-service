package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
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

func (s *ReservationStore) Fail(ctx context.Context, id uuid.UUID, createdAt time.Time) error {
	return finish(ctx, s.queries, id, createdAt, reservation.StatusFailed, nil)
}

// Transact locks the wallets in a fixed order, so two transactions touching
// the same pair cannot deadlock. The advisory locks end with the transaction.
func (s *ReservationStore) Transact(ctx context.Context, walletIDs []string, fn func(reservation.Tx) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := codegen.New(tx)
	ids := slices.Clone(walletIDs)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		if err := q.LockWallet(ctx, id); err != nil {
			return fmt.Errorf("lock wallet %s: %w", id, err)
		}
	}
	if err := fn(&reservationTx{queries: q}); err != nil {
		return mapDBError(err)
	}
	return tx.Commit(ctx)
}

type reservationTx struct {
	queries *codegen.Queries
}

func (t *reservationTx) Usage(ctx context.Context, key reservation.UsageKey) (decimal.Decimal, error) {
	value, err := t.queries.GetUsageValue(ctx, codegen.GetUsageValueParams{
		WalletID:        key.WalletID,
		PolicyTypeID:    key.PolicyTypeID,
		TransactionType: key.TransactionType,
		Channel:         key.Channel,
		Direction:       string(key.Direction),
		WindowType:      string(key.WindowType),
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

var _ reservation.Store = (*ReservationStore)(nil)
