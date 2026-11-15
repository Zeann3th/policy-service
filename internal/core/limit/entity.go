package limit

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

const All = "ALL"

type Operator string

const (
	OpEQ  Operator = "EQ"
	OpNEQ Operator = "NEQ"
	OpGT  Operator = "GT"
	OpGTE Operator = "GTE"
	OpLT  Operator = "LT"
	OpLTE Operator = "LTE"
)

func (o Operator) Valid() bool {
	switch o {
	case OpEQ, OpNEQ, OpGT, OpGTE, OpLT, OpLTE:
		return true
	}
	return false
}

func (o Operator) Satisfied(actual, bound decimal.Decimal) bool {
	switch o {
	case OpEQ:
		return actual.Equal(bound)
	case OpNEQ:
		return !actual.Equal(bound)
	case OpGT:
		return actual.GreaterThan(bound)
	case OpGTE:
		return actual.GreaterThanOrEqual(bound)
	case OpLT:
		return actual.LessThan(bound)
	case OpLTE:
		return actual.LessThanOrEqual(bound)
	}
	return false
}

type Unit struct {
	ID        int64
	Code      string
	Active    bool
	CreatedAt time.Time
	UpdatedAt *time.Time
}

type LimitType struct {
	ID          int64
	Code        string
	Description *string
	UnitID      int64
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

type LimitGroup struct {
	ID         int64
	WalletType string
	Currency   string
	Tier       string
	CreatedAt  time.Time
	UpdatedAt  *time.Time
}

type LimitKey struct {
	LimitTypeID     int64
	TransactionType string
	Channel         string
}

type LimitValue struct {
	ID              int64
	LimitGroupID    int64
	LimitTypeID     int64
	LimitType       string
	UnitID          int64
	TransactionType string
	Channel         string
	Operator        Operator
	Value           decimal.Decimal
	Source          Source
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

func (v LimitValue) Key() LimitKey {
	return LimitKey{LimitTypeID: v.LimitTypeID, TransactionType: v.TransactionType, Channel: v.Channel}
}

type UserLimit struct {
	ID              int64
	WalletID        string
	LimitTypeID     int64
	LimitType       string
	UnitID          int64
	TransactionType string
	Channel         string
	Operator        Operator
	Value           decimal.Decimal
	EffectiveFrom   time.Time
	EffectiveTo     *time.Time
	Reason          *string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

func (v UserLimit) Key() LimitKey {
	return LimitKey{LimitTypeID: v.LimitTypeID, TransactionType: v.TransactionType, Channel: v.Channel}
}

type AuditAction string

const (
	AuditInsert AuditAction = "INSERT"
	AuditUpdate AuditAction = "UPDATE"
	AuditDelete AuditAction = "DELETE"
)

type AuditLog struct {
	ID        int64
	Entity    string // limit_type | limit_group | limit_value | user_limit
	EntityKey string
	Action    AuditAction
	OldValue  json.RawMessage
	NewValue  json.RawMessage
	UpdatedBy string
	UpdatedAt time.Time
}

type Source string

const (
	SourceSystem Source = "SYSTEM"
	SourceUser   Source = "USER"
)

type ResolvedLimit struct {
	LimitType       string
	TransactionType string
	Channel         string
	Operator        Operator
	Value           decimal.Decimal
	Source          Source
}
