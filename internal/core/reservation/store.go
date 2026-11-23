package reservation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type Store interface {
	// Create inserts a PROCESSING reservation and returns it with CreatedAt.
	// A duplicate request id in the same partition is core.ErrConflict.
	Create(context.Context, Reservation) (Reservation, error)
	// Fail marks a PROCESSING reservation FAILED.
	Fail(ctx context.Context, id uuid.UUID, createdAt time.Time) error
	// Transact runs fn in one database transaction holding an exclusive lock
	// per wallet; it commits when fn returns nil and rolls back otherwise.
	Transact(ctx context.Context, walletIDs []string, fn func(Tx) error) error
}

type Tx interface {
	Usage(context.Context, UsageKey) (decimal.Decimal, error)
	// Apply adds the entry's delta to its usages row, records the entry in
	// the ledger and returns the usage value after the change.
	Apply(context.Context, LedgerEntry) (decimal.Decimal, error)
	// Finish moves a PROCESSING reservation to status and stores result.
	Finish(ctx context.Context, id uuid.UUID, createdAt time.Time, status Status, result json.RawMessage) error
}

type PolicyResolver interface {
	Resolve(context.Context, policy.ResolveRequest) ([]policy.Policy, error)
}
