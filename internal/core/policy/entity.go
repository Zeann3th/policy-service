package policy

import (
	"encoding/json"
	"slices"
	"time"

	"github.com/shopspring/decimal"
)

const All = "ALL"

// Kind says which strategy evaluates a policy. It is set on each policy
// row; policy types and units are only labels.
type Kind string

const (
	KindLimit     Kind = "LIMIT"     // blocks a transaction that breaks a bound
	KindWaiver    Kind = "WAIVER"    // waives the fee while usage is under a bound
	KindAuxiliary Kind = "AUXILIARY" // charges: fee, discount, tax, by role
)

var kinds = []Kind{KindLimit, KindWaiver, KindAuxiliary}

func (k Kind) Valid() bool {
	return slices.Contains(kinds, k)
}

// Kinds returns every kind, in evaluation order.
func Kinds() []Kind {
	return slices.Clone(kinds)
}

// Role is how an AUXILIARY policy's charge applies. Charges run in role
// order, so a role may only use a base computed before it.
type Role string

const (
	RoleFee      Role = "FEE"      // paid; zero while a waiver applies; publishes base FEE
	RoleDiscount Role = "DISCOUNT" // taken off; at most the fee (or the amount, for cashback); publishes NET_FEE
	RoleTax      Role = "TAX"      // paid, on any base
)

var roles = []Role{RoleFee, RoleDiscount, RoleTax}

func (r Role) Valid() bool {
	return slices.Contains(roles, r)
}

// Roles returns every role, in the order charges are computed.
func Roles() []Role {
	return slices.Clone(roles)
}

// Unit codes the code has a measure for. Units are catalog rows (table
// units); the engine does nothing with a policy in another unit.
const (
	UnitAmount = "AMOUNT" // money: the transaction amount, or the amount charged
	UnitCount  = "COUNT"  // number of transactions
)

// Base is what an AUXILIARY value is a percentage of; no base means the
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
	Role     Role   // AUXILIARY only
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
	Kind            Kind
	Role            Role
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
	Kind            Kind
	Role            Role
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
	return Spec{Kind: v.Kind, Role: v.Role, Unit: v.Unit, Base: v.Base, Operator: v.Operator, Window: v.WindowType, Value: v.Value, Cap: v.Cap}
}
