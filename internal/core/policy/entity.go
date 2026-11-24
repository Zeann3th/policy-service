package policy

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

const All = "ALL"

// Kind says which strategy evaluates a policy. It is set on the policy
// type, so every policy of a type behaves alike; units are labels. The
// charge kinds (FEE, DISCOUNT, TAX) run in this order, so a charge may only
// use a base computed before it.
type Kind string

const (
	KindLimit    Kind = "LIMIT"    // blocks a transaction that breaks a bound
	KindWaiver   Kind = "WAIVER"   // waives the fee while usage is under a bound
	KindFee      Kind = "FEE"      // paid; zero while a waiver applies; publishes base FEE
	KindDiscount Kind = "DISCOUNT" // taken off; at most the fee (or the amount, for cashback); publishes NET_FEE
	KindTax      Kind = "TAX"      // paid, on any base
)

var kinds = []Kind{KindLimit, KindWaiver, KindFee, KindDiscount, KindTax}

func (k Kind) Valid() bool {
	return slices.Contains(kinds, k)
}

// Kinds returns every kind, in evaluation order.
func Kinds() []Kind {
	return slices.Clone(kinds)
}

// Unit codes the code has a measure for. Units are catalog rows (table
// units); the engine does nothing with a policy in another unit.
const (
	UnitAmount = "AMOUNT" // money: the transaction amount, or the amount charged
	UnitCount  = "COUNT"  // number of transactions
)

// Base is what a charge (FEE, DISCOUNT, TAX) is a percentage of; no base means the
// value is a flat amount. Charges are computed in type order (fee,
// discount, tax), so a type may only use a base computed before it.
type Base string

const (
	BaseAmount Base = "AMOUNT"  // the transaction amount
	BaseFee    Base = "FEE"     // the total fee
	BaseNetFee Base = "NET_FEE" // the fee minus fee-based discounts
)

var bases = []Base{BaseAmount, BaseFee, BaseNetFee}

func (b Base) Valid() bool {
	return slices.Contains(bases, b)
}

// Bases returns every base, in the order they become available.
func Bases() []Base {
	return slices.Clone(bases)
}

// Spec is how a policy reads its value. The kind's strategy decides which
// fields it takes and checks them (Rules).
type Spec struct {
	Kind     Kind
	Unit     string // unit code; "" when the policy has none
	Base     Base
	Operator Operator
	Window   WindowType
	Value    decimal.Decimal
	Cap      *decimal.Decimal
}

// Rules checks the value fields a policy's kind takes. It is implemented
// next to the evaluation strategies.
type Rules interface {
	Check(Spec) error
}

type WindowType string

const (
	WindowTransaction WindowType = "TRANSACTION" // this transaction alone; nothing accumulates
	WindowDaily       WindowType = "DAILY"
	WindowMonthly     WindowType = "MONTHLY"
	WindowLifetime    WindowType = "LIFETIME"
)

var windows = []WindowType{WindowTransaction, WindowDaily, WindowMonthly, WindowLifetime}

func (w WindowType) Valid() bool {
	return slices.Contains(windows, w)
}

// Windows returns every window, shortest first.
func Windows() []WindowType {
	return slices.Clone(windows)
}

type Direction string

const (
	DirectionAll Direction = "ALL"
	DirectionOut Direction = "OUT"
	DirectionIn  Direction = "IN"
)

var directions = []Direction{DirectionAll, DirectionOut, DirectionIn}

func (d Direction) Valid() bool {
	return slices.Contains(directions, d)
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

var operators = []Operator{OpEQ, OpNEQ, OpGT, OpGTE, OpLT, OpLTE}

// comparisons is how each operator compares the actual value to the bound.
var comparisons = map[Operator]func(actual, bound decimal.Decimal) bool{
	OpEQ:  decimal.Decimal.Equal,
	OpNEQ: func(actual, bound decimal.Decimal) bool { return !actual.Equal(bound) },
	OpGT:  decimal.Decimal.GreaterThan,
	OpGTE: decimal.Decimal.GreaterThanOrEqual,
	OpLT:  decimal.Decimal.LessThan,
	OpLTE: decimal.Decimal.LessThanOrEqual,
}

func (o Operator) Valid() bool {
	_, ok := comparisons[o]
	return ok
}

// Operators returns every operator.
func Operators() []Operator {
	return slices.Clone(operators)
}

// Satisfied reports whether actual compares to bound as the operator says;
// an unknown operator is never satisfied.
func (o Operator) Satisfied(actual, bound decimal.Decimal) bool {
	compare, ok := comparisons[o]
	return ok && compare(actual, bound)
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
	UnitID          int64 // 0 when the policy has no unit
}

type Policy struct {
	ID              int64
	PolicyGroupID   int64
	PolicyTypeID    int64
	PolicyType      string
	Kind            Kind // from the type
	TransactionType string
	Channel         string
	Direction       Direction
	WindowType      WindowType
	UnitID          *int64
	Unit            string // unit code, joined from units
	Base            Base
	Operator        Operator
	Value           decimal.Decimal
	Cap             *decimal.Decimal
	// IsEnforced false keeps the policy out of the engine; it is still
	// returned as config.
	IsEnforced    bool
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	Source        Source
	CreatedAt     time.Time
	UpdatedAt     *time.Time
}

func (v Policy) Key() PolicyKey {
	return PolicyKey{PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: v.Direction, WindowType: v.WindowType, UnitID: unitID(v.UnitID)}
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
	Kind            Kind // from the type
	TransactionType string
	Channel         string
	Direction       Direction
	WindowType      WindowType
	UnitID          *int64
	Unit            string // unit code, joined from units
	Base            Base
	Operator        Operator
	Value           decimal.Decimal
	Cap             *decimal.Decimal
	// IsEnforced false keeps the policy out of the engine; it is still
	// returned as config.
	IsEnforced    bool
	EffectiveFrom time.Time
	EffectiveTo   *time.Time
	Reason        *string
	CreatedBy     string
	CreatedAt     time.Time
	UpdatedAt     *time.Time
}

func (v CustomPolicy) Key() PolicyKey {
	return PolicyKey{PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: v.Direction, WindowType: v.WindowType, UnitID: unitID(v.UnitID)}
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

func unitID(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

// Spec is how the policy reads its value.
func (v Policy) Spec() Spec {
	return Spec{Kind: v.Kind, Unit: v.Unit, Base: v.Base, Operator: v.Operator, Window: v.WindowType, Value: v.Value, Cap: v.Cap}
}
