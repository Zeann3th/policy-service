package reservation

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type PartyRequest struct {
	WalletID   string `binding:"required" example:"wallet-123" json:"walletId"`
	WalletType string `binding:"required" example:"PERSONAL" json:"walletType"`
	Tier       string `binding:"required" example:"TIER_1" json:"tier"`
}

type ReserveRequest struct {
	RequestID       string           `binding:"required" example:"0193a7e2-5c1f-7b3a-9d2e-1f4c5b6a7d8e" json:"requestId"`
	TransactionType string           `binding:"required" example:"TRANSFER" json:"transactionType"`
	Channel         string           `example:"APP" json:"channel"`
	Currency        string           `binding:"required,iso4217" example:"VND" json:"currency"`
	Amount          *decimal.Decimal `binding:"required" example:"100000" json:"amount" swaggertype:"string"`
	Sender          PartyRequest     `binding:"required" json:"sender"`
	Receiver        PartyRequest     `binding:"required" json:"receiver"`
}

type ReserveResponse struct {
	ReservationID uuid.UUID   `json:"reservationId" swaggertype:"string"`
	RequestID     string      `json:"requestId"`
	Status        Status      `enums:"RESERVED,REJECTED" json:"status"`
	Sender        PartyResult `json:"sender"`
	Receiver      PartyResult `json:"receiver"`
	ResponseTime  time.Time   `format:"date-time" json:"responseTime"`
}

type ReleaseResponse struct {
	ReservationID uuid.UUID `json:"reservationId" swaggertype:"string"`
	RequestID     string    `json:"requestId"`
	Status        Status    `enums:"RELEASED" json:"status"`
	ResponseTime  time.Time `format:"date-time" json:"responseTime"`
}

type PartyResult struct {
	WalletID   string `json:"walletId"`
	IsViolated bool   `json:"isViolated"`
	// Charges is what each AUXILIARY role charges this side, in role order;
	// a discount is negative. The side pays their sum.
	Charges         []Charge        `json:"charges"`
	AppliedPolicies []AppliedPolicy `json:"appliedPolicies"`
}

// Charge is one role's total for a side: positive is paid, negative (a
// discount) is taken off.
type Charge struct {
	Role   policy.Role     `enums:"FEE,DISCOUNT,TAX" json:"role"`
	Amount decimal.Decimal `example:"1100" json:"amount" swaggertype:"string"`
}

type AppliedPolicy struct {
	PolicyID        int64             `json:"policyId"`
	PolicyType      string            `json:"policyType"`
	Kind            policy.Kind       `enums:"LIMIT,WAIVER,AUXILIARY" json:"kind"`
	Role            policy.Role       `enums:"FEE,DISCOUNT,TAX" json:"role,omitempty"`
	Unit            string            `example:"AMOUNT" json:"unit,omitempty"`
	Base            policy.Base       `enums:"AMOUNT,FEE,NET_FEE" json:"base,omitempty"`
	Source          policy.Source     `enums:"SYSTEM,USER" json:"source"`
	TransactionType string            `json:"transactionType"`
	Channel         string            `json:"channel"`
	Direction       policy.Direction  `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      policy.WindowType `enums:"TRANSACTION,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	WindowStart     string            `example:"2026-11-23" json:"windowStart,omitempty"`
	Operator        policy.Operator   `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator,omitempty"`
	Value           decimal.Decimal   `json:"value" swaggertype:"string"`
	Cap             *decimal.Decimal  `json:"cap,omitempty" swaggertype:"string"`
	// Usage is the window total including this request (not stored when rejected).
	Usage *decimal.Decimal `json:"usage,omitempty" swaggertype:"string"`
	// Charge is what this AUXILIARY line charges, signed like Charges, after
	// its window cap and before the type's own caps (a discount at most the
	// fee) are applied to the totals.
	Charge   *decimal.Decimal `json:"charge,omitempty" swaggertype:"string"`
	Violated bool             `json:"violated"`
}
