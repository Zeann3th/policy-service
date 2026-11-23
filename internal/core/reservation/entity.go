package reservation

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type Status string

const (
	StatusPending   Status = "PENDING"
	StatusReserved  Status = "RESERVED"
	StatusCommitted Status = "COMMITTED"
	StatusReleased  Status = "RELEASED"
	StatusExpired   Status = "EXPIRED"
	StatusRejected  Status = "REJECTED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusReserved, StatusCommitted, StatusReleased, StatusExpired, StatusRejected:
		return true
	}
	return false
}

type CommitSource string

const (
	CommitSourceEvent CommitSource = "EVENT"
	CommitSourceAuto  CommitSource = "AUTO"
)

func (c CommitSource) Valid() bool {
	switch c {
	case CommitSourceEvent, CommitSourceAuto:
		return true
	}
	return false
}

// Reservation is one request. ID is a UUIDv7 minted by the application;
// RequestID is the caller's id and is unique.
type Reservation struct {
	ID               uuid.UUID
	RequestID        string
	Status           Status
	CommitSource     *CommitSource
	TransactionType  string
	Channel          string
	Currency         string
	Amount           decimal.Decimal
	SenderWalletID   string
	ReceiverWalletID string
	Result           json.RawMessage // null while PENDING
	ExpiresAt        *time.Time      // set when it becomes RESERVED
	CreatedAt        time.Time
	UpdatedAt        *time.Time
}

// UsageKey identifies a usages row. It is built from the selectors of the
// matched policy, never from a policy id, so usage outlives policy edits.
type UsageKey struct {
	WalletID        string
	PolicyTypeID    int64
	TransactionType string
	Channel         string
	Direction       policy.Direction
	WindowType      policy.WindowType // never WindowNone
	WindowStart     time.Time         // the day, the first day of the month, or 1970-01-01
}

// Usage is the accumulated total for a UsageKey, in the unit of the policy type.
type Usage struct {
	UsageKey
	Value decimal.Decimal
}

// LedgerEntry is one immutable step into a Usage: what a reservation added
// to that UsageKey, plus a snapshot of the policy that applied at the time.
type LedgerEntry struct {
	ReservationID uuid.UUID
	UsageKey
	Delta          decimal.Decimal // 1 for COUNT, the amount for AMOUNT
	PolicyID       *int64          // id in policies or custom_policies
	PolicyTypeCode string
	Operator       *policy.Operator
	LimitValue     *decimal.Decimal
	ValueAfter     decimal.Decimal
	CreatedAt      time.Time
}
