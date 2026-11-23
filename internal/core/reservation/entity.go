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
	StatusProcessing Status = "PROCESSING"
	StatusReserved   Status = "RESERVED"
	StatusReleased   Status = "RELEASED"
	StatusRejected   Status = "REJECTED"
	StatusFailed     Status = "FAILED"
)

func (s Status) Valid() bool {
	switch s {
	case StatusProcessing, StatusReserved, StatusReleased, StatusRejected, StatusFailed:
		return true
	}
	return false
}

type EntryType string

const (
	EntryReserve EntryType = "RESERVE"
	EntryRelease EntryType = "RELEASE"
)

func (e EntryType) Valid() bool {
	switch e {
	case EntryReserve, EntryRelease:
		return true
	}
	return false
}

type Party struct {
	WalletID   string
	WalletType string
	Tier       string
}

type Reservation struct {
	ID              uuid.UUID
	RequestID       string
	Status          Status
	TransactionType string
	Channel         string
	Currency        string
	Amount          decimal.Decimal
	Sender          Party
	Receiver        Party
	Result          json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

type UsageKey struct {
	WalletID        string
	PolicyTypeID    int64
	TransactionType string
	Channel         string
	Direction       policy.Direction
	WindowType      policy.WindowType
	UnitID          int64
	WindowStart     time.Time
}

type Usage struct {
	UsageKey
	Value decimal.Decimal
}

type LedgerEntry struct {
	ReservationID        uuid.UUID
	ReservationCreatedAt time.Time
	EntryType            EntryType
	UsageKey
	Delta          decimal.Decimal
	PolicyID       *int64
	PolicyTypeCode string
	Operator       *policy.Operator
	LimitValue     *decimal.Decimal
	ValueAfter     decimal.Decimal
	CreatedAt      time.Time
}
