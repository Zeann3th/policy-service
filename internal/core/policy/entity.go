package policy

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

const All = "ALL"

type Kind string

const (
	KindLimit    Kind = "LIMIT"
	KindWaiver   Kind = "WAIVER"
	KindFee      Kind = "FEE"
	KindTax      Kind = "TAX"
	KindDiscount Kind = "DISCOUNT"
)

func (k Kind) Valid() bool {
	switch k {
	case KindLimit, KindWaiver, KindFee, KindTax, KindDiscount:
		return true
	}
	return false
}

type WindowType string

const (
	WindowNone     WindowType = "NONE"
	WindowDaily    WindowType = "DAILY"
	WindowMonthly  WindowType = "MONTHLY"
	WindowLifetime WindowType = "LIFETIME"
)

func (w WindowType) Valid() bool {
	switch w {
	case WindowNone, WindowDaily, WindowMonthly, WindowLifetime:
		return true
	}
	return false
}

type Direction string

const (
	DirectionAll Direction = "ALL"
	DirectionOut Direction = "OUT"
	DirectionIn  Direction = "IN"
)

func (d Direction) Valid() bool {
	switch d {
	case DirectionAll, DirectionOut, DirectionIn:
		return true
	}
	return false
}

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

type PolicyType struct {
	ID          int64
	Code        string
	Description *string
	UnitID      int64
	Kind        Kind
	WindowType  WindowType
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   *time.Time
}

type PolicyGroup struct {
	ID         int64
	WalletType string
	Currency   string
	Tier       string
	CreatedAt  time.Time
	UpdatedAt  *time.Time
}

type PolicyKey struct {
	PolicyTypeID    int64
	TransactionType string
	Channel         string
	Direction       Direction
}

type Policy struct {
	ID              int64
	PolicyGroupID   int64
	PolicyTypeID    int64
	PolicyType      string
	UnitID          int64
	TransactionType string
	Channel         string
	Direction       Direction
	Operator        Operator
	Value           decimal.Decimal
	Source          Source
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

func (v Policy) Key() PolicyKey {
	return PolicyKey{PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: v.Direction}
}

type CustomPolicy struct {
	ID              int64
	WalletID        string
	PolicyTypeID    int64
	PolicyType      string
	UnitID          int64
	TransactionType string
	Channel         string
	Direction       Direction
	Operator        Operator
	Value           decimal.Decimal
	EffectiveFrom   time.Time
	EffectiveTo     *time.Time
	Reason          *string
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

func (v CustomPolicy) Key() PolicyKey {
	return PolicyKey{PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: v.Direction}
}

type AuditAction string

const (
	AuditInsert AuditAction = "INSERT"
	AuditUpdate AuditAction = "UPDATE"
	AuditDelete AuditAction = "DELETE"
)

type AuditLog struct {
	ID        int64
	Entity    string // policy_type | policy_group | policy | custom_policy
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
	SourceCustom Source = "USER"
)
