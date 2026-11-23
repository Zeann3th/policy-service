package policy

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
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
	_, ok := shapes[k]
	return ok
}

// Kinds returns every declared kind.
func Kinds() []Kind {
	return slices.Sorted(maps.Keys(shapes))
}

// Unit is what a LIMIT or WAIVER measures.
type Unit string

const (
	UnitAmount  Unit = "AMOUNT"  // the transaction amount
	UnitCount   Unit = "COUNT"   // the number of transactions
	UnitBalance Unit = "BALANCE" // the wallet balance (not evaluated on reserve)
)

func (u Unit) Valid() bool {
	return slices.Contains([]Unit{UnitAmount, UnitCount, UnitBalance}, u)
}

// Base is what a FEE, DISCOUNT or TAX value is a percentage of; no base
// means the value is a flat amount. Charges are computed in the order FEE,
// DISCOUNT, TAX, so a kind may only use a base computed before it.
type Base string

const (
	BaseAmount Base = "AMOUNT"  // the transaction amount
	BaseFee    Base = "FEE"     // the total fee
	BaseNetFee Base = "NET_FEE" // the fee minus fee-based discounts
)

func (b Base) Valid() bool {
	return slices.Contains([]Base{BaseAmount, BaseFee, BaseNetFee}, b)
}

// shape is which value fields a policy of a kind uses. Declaring a kind here
// makes it valid for policy types; its evaluation is a strategy registered
// in package reservation.
type shape struct {
	units    []Unit // required when not empty, otherwise absent
	bases    []Base // optional when not empty, otherwise absent
	operator bool   // required when true, otherwise absent
	windowed bool   // may accumulate over a window
}

var shapes = map[Kind]shape{
	KindLimit:    {units: []Unit{UnitAmount, UnitCount, UnitBalance}, operator: true, windowed: true},
	KindWaiver:   {units: []Unit{UnitCount}, windowed: true},
	KindFee:      {bases: []Base{BaseAmount}},
	KindDiscount: {bases: []Base{BaseAmount, BaseFee}},
	KindTax:      {bases: []Base{BaseAmount, BaseFee, BaseNetFee}},
}

// Check reports the first value field that does not fit kind, or nil.
func (k Kind) Check(unit Unit, base Base, operator Operator, window WindowType) error {
	s, ok := shapes[k]
	switch {
	case !ok:
		return fmt.Errorf("unknown kind %q", k)
	case len(s.units) == 0 && unit != "":
		return fmt.Errorf("%s policies take no unit", k)
	case len(s.units) > 0 && !slices.Contains(s.units, unit):
		return fmt.Errorf("%s policies need unit %v", k, s.units)
	case base != "" && !slices.Contains(s.bases, base):
		return fmt.Errorf("%s policies take base %v or none", k, s.bases)
	case !s.operator && operator != "":
		return fmt.Errorf("%s policies take no operator", k)
	case s.operator && !operator.Valid():
		return fmt.Errorf("%s policies need an operator", k)
	case !s.windowed && window != WindowNone:
		return fmt.Errorf("%s policies take window %s only", k, WindowNone)
	}
	return nil
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
	Kind        Kind
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

// PolicyKey is a policy's identity: a custom policy with the same key
// overrides the system one.
type PolicyKey struct {
	PolicyTypeID    int64
	TransactionType string
	Channel         string
	Direction       Direction
	WindowType      WindowType
	Unit            Unit
}

type Policy struct {
	ID              int64
	PolicyGroupID   int64
	PolicyTypeID    int64
	PolicyType      string
	Kind            Kind
	TransactionType string
	Channel         string
	Direction       Direction
	WindowType      WindowType
	Unit            Unit
	Base            Base
	Operator        Operator
	Value           decimal.Decimal
	EffectiveFrom   time.Time
	EffectiveTo     *time.Time
	Source          Source
	CreatedAt       time.Time
	UpdatedAt       *time.Time
}

func (v Policy) Key() PolicyKey {
	return PolicyKey{PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: v.Direction, WindowType: v.WindowType, Unit: v.Unit}
}

// Matches reports whether the policy's selectors cover a transaction.
func (v Policy) Matches(transactionType, channel string, direction Direction) bool {
	return (v.TransactionType == All || v.TransactionType == transactionType) &&
		(v.Channel == All || v.Channel == channel) &&
		(v.Direction == DirectionAll || v.Direction == direction)
}

type CustomPolicy struct {
	ID              int64
	WalletID        string
	PolicyTypeID    int64
	PolicyType      string
	Kind            Kind
	TransactionType string
	Channel         string
	Direction       Direction
	WindowType      WindowType
	Unit            Unit
	Base            Base
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
	return PolicyKey{PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: v.Direction, WindowType: v.WindowType, Unit: v.Unit}
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
