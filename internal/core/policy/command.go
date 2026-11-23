package policy

import (
	"time"

	"github.com/shopspring/decimal"
)

type PolicyTypeResponse struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Description *string    `json:"description,omitempty"`
	Kind        Kind       `enums:"LIMIT,WAIVER,AUXILIARY" json:"kind"`
	IsEnforced  bool       `json:"isEnforced"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `format:"date-time" json:"createdAt"`
	UpdatedAt   *time.Time `format:"date-time" json:"updatedAt,omitempty"`
}

type CreatePolicyTypeRequest struct {
	Code        string  `binding:"required" json:"code"`
	Description *string `json:"description"`
	// Kind defaults to LIMIT.
	Kind Kind `enums:"LIMIT,WAIVER,AUXILIARY" json:"kind"`
	// IsEnforced defaults to true; false stores and serves the type's
	// policies without evaluating them on reserve.
	IsEnforced *bool  `json:"isEnforced"`
	Active     *bool  `json:"active"`
	UpdatedBy  string `binding:"required" json:"updatedBy"`
}

type UpdatePolicyTypeRequest struct {
	Description *string `json:"description"`
	// IsEnforced keeps the current value when absent.
	IsEnforced *bool  `json:"isEnforced"`
	Active     *bool  `binding:"required" json:"active"`
	UpdatedBy  string `binding:"required" json:"updatedBy"`
}

type PolicyGroupResponse struct {
	ID         int64      `json:"id"`
	WalletType string     `json:"walletType"`
	Currency   string     `json:"currency"`
	Tier       string     `json:"tier"`
	CreatedAt  time.Time  `format:"date-time" json:"createdAt"`
	UpdatedAt  *time.Time `format:"date-time" json:"updatedAt,omitempty"`
}

type CreatePolicyGroupRequest struct {
	WalletType string `binding:"required" json:"walletType"`
	Currency   string `binding:"required,iso4217" json:"currency"`
	Tier       string `binding:"required" json:"tier"`
	UpdatedBy  string `binding:"required" json:"updatedBy"`
}

type PolicyResponse struct {
	ID              int64            `json:"id"`
	PolicyGroupID   int64            `json:"policyGroupId"`
	PolicyTypeID    int64            `json:"policyTypeId"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Direction       Direction        `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType       `enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	UnitID          *int64           `json:"unitId,omitempty"`
	Base            Base             `enums:"AMOUNT,FEE,NET_FEE" json:"base,omitempty"`
	Operator        Operator         `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator,omitempty"`
	Value           decimal.Decimal  `example:"10000" json:"value" swaggertype:"string"`
	Cap             *decimal.Decimal `json:"cap,omitempty" swaggertype:"string"`
	EffectiveFrom   time.Time        `format:"date-time" json:"effectiveFrom"`
	EffectiveTo     *time.Time       `format:"date-time" json:"effectiveTo,omitempty"`
	CreatedAt       time.Time        `format:"date-time" json:"createdAt"`
	UpdatedAt       *time.Time       `format:"date-time" json:"updatedAt,omitempty"`
}

// PolicyFields are what a system or custom policy is created with.
type PolicyFields struct {
	PolicyTypeID    int64      `binding:"required,gt=0" json:"policyTypeId"`
	TransactionType string     `json:"transactionType"`
	Channel         string     `json:"channel"`
	Direction       Direction  `binding:"omitempty,oneof=ALL OUT IN" enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType `binding:"omitempty,oneof=NONE DAILY MONTHLY LIFETIME" enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	// UnitID (units table) is what a LIMIT or WAIVER measures, or what an
	// AUXILIARY cap counts; required for LIMIT and WAIVER.
	UnitID *int64 `json:"unitId"`
	PolicyChange
}

// PolicyChange is what an existing policy may change; its type, scope,
// window and unit are its identity and never change.
type PolicyChange struct {
	// Base makes an AUXILIARY value a percentage of it; absent: flat amount.
	Base Base `binding:"omitempty,oneof=AMOUNT FEE NET_FEE" enums:"AMOUNT,FEE,NET_FEE" json:"base"`
	// Operator compares a LIMIT; required for LIMIT, absent otherwise.
	Operator Operator         `binding:"omitempty,oneof=EQ NEQ GT GTE LT LTE" enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value    *decimal.Decimal `binding:"required" example:"10000" json:"value" swaggertype:"string"`
	// Cap is the most an AUXILIARY policy charges per window, in its unit;
	// required with a window, absent otherwise.
	Cap *decimal.Decimal `example:"50000" json:"cap" swaggertype:"string"`
	// EffectiveFrom defaults to now on create and is required on update.
	EffectiveFrom *time.Time `format:"date-time" json:"effectiveFrom"`
	EffectiveTo   *time.Time `format:"date-time" json:"effectiveTo"`
}

type CreatePolicyRequest struct {
	PolicyFields
	UpdatedBy string `binding:"required" json:"updatedBy"`
}

type UpdatePolicyRequest struct {
	PolicyChange
	UpdatedBy string `binding:"required" json:"updatedBy"`
}

type CustomPolicyResponse struct {
	ID              int64            `json:"id"`
	WalletID        string           `json:"walletId"`
	PolicyTypeID    int64            `json:"policyTypeId"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Direction       Direction        `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType       `enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	UnitID          *int64           `json:"unitId,omitempty"`
	Base            Base             `enums:"AMOUNT,FEE,NET_FEE" json:"base,omitempty"`
	Operator        Operator         `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator,omitempty"`
	Value           decimal.Decimal  `example:"10000" json:"value" swaggertype:"string"`
	Cap             *decimal.Decimal `json:"cap,omitempty" swaggertype:"string"`
	EffectiveFrom   time.Time        `format:"date-time" json:"effectiveFrom"`
	EffectiveTo     *time.Time       `format:"date-time" json:"effectiveTo,omitempty"`
	Reason          *string          `json:"reason,omitempty"`
	CreatedBy       string           `json:"createdBy"`
	CreatedAt       time.Time        `format:"date-time" json:"createdAt"`
	UpdatedAt       *time.Time       `format:"date-time" json:"updatedAt,omitempty"`
}

type CreateCustomPolicyRequest struct {
	PolicyFields
	Reason    *string `json:"reason"`
	UpdatedBy string  `binding:"required" json:"updatedBy"`
}

type UpdateCustomPolicyRequest struct {
	PolicyChange
	Reason    *string `json:"reason"`
	UpdatedBy string  `binding:"required" json:"updatedBy"`
}

type GetConfigRequest struct {
	WalletType      string `binding:"required" form:"walletType" json:"walletType"`
	Currency        string `binding:"required,iso4217" form:"currency" json:"currency"`
	Tier            string `binding:"required" form:"tier" json:"tier"`
	WalletID        string `binding:"required" form:"walletId" json:"walletId"`
	TransactionType string `form:"transactionType" json:"transactionType,omitempty"`
}

type ResolveRequest struct {
	WalletID        string
	WalletType      string
	Currency        string
	Tier            string
	TransactionType string
	Channel         string
	Direction       Direction
}

type GetConfigResponse struct {
	Rules []Rule `json:"rules"`
}

type Rule struct {
	PolicyType      string           `json:"policyType"`
	Kind            Kind             `enums:"LIMIT,WAIVER,AUXILIARY" json:"kind"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Direction       Direction        `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType       `enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	Unit            string           `example:"AMOUNT" json:"unit,omitempty"`
	Base            Base             `enums:"AMOUNT,FEE,NET_FEE" json:"base,omitempty"`
	Operator        Operator         `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator,omitempty"`
	Value           decimal.Decimal  `example:"10000" json:"value" swaggertype:"string"`
	Cap             *decimal.Decimal `json:"cap,omitempty" swaggertype:"string"`
	Source          Source           `enums:"SYSTEM,USER" json:"source"`
}
