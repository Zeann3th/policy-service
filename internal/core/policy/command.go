package policy

import (
	"time"

	"github.com/shopspring/decimal"
)

type PolicyTypeResponse struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Description *string    `json:"description,omitempty"`
	UnitID      int64      `json:"unitId"`
	Kind        Kind       `enums:"LIMIT,WAIVER,FEE,TAX,DISCOUNT" json:"kind"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `format:"date-time" json:"createdAt"`
	UpdatedAt   *time.Time `format:"date-time" json:"updatedAt,omitempty"`
}

type CreatePolicyTypeRequest struct {
	Code        string  `binding:"required" json:"code"`
	Description *string `json:"description"`
	UnitID      int64   `binding:"required,gt=0" json:"unitId"`
	Kind        Kind    `binding:"omitempty,oneof=LIMIT WAIVER FEE TAX DISCOUNT" enums:"LIMIT,WAIVER,FEE,TAX,DISCOUNT" json:"kind"`
	Active      *bool   `json:"active"`
	UpdatedBy   string  `binding:"required" json:"updatedBy"`
}

type UpdatePolicyTypeRequest struct {
	Description *string `json:"description"`
	Active      *bool   `binding:"required" json:"active"`
	UpdatedBy   string  `binding:"required" json:"updatedBy"`
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
	ID              int64           `json:"id"`
	PolicyGroupID   int64           `json:"policyGroupId"`
	PolicyTypeID    int64           `json:"policyTypeId"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Direction       Direction       `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType      `enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	Operator        Operator        `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value           decimal.Decimal `example:"10000" json:"value" swaggertype:"string"`
	CreatedAt       time.Time       `format:"date-time" json:"createdAt"`
	UpdatedAt       *time.Time      `format:"date-time" json:"updatedAt,omitempty"`
}

type CreatePolicyRequest struct {
	PolicyTypeID    int64            `binding:"required,gt=0" json:"policyTypeId"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Direction       Direction        `binding:"omitempty,oneof=ALL OUT IN" enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType       `binding:"omitempty,oneof=NONE DAILY MONTHLY LIFETIME" enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	Operator        Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value           *decimal.Decimal `binding:"required" example:"10000" json:"value" swaggertype:"string"`
	UpdatedBy       string           `binding:"required" json:"updatedBy"`
}

type UpdatePolicyRequest struct {
	Operator  Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value     *decimal.Decimal `binding:"required" example:"10000" json:"value" swaggertype:"string"`
	UpdatedBy string           `binding:"required" json:"updatedBy"`
}

type CustomPolicyResponse struct {
	ID              int64           `json:"id"`
	WalletID        string          `json:"walletId"`
	PolicyTypeID    int64           `json:"policyTypeId"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Direction       Direction       `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType      `enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	Operator        Operator        `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value           decimal.Decimal `example:"10000" json:"value" swaggertype:"string"`
	EffectiveFrom   time.Time       `format:"date-time" json:"effectiveFrom"`
	EffectiveTo     *time.Time      `format:"date-time" json:"effectiveTo,omitempty"`
	Reason          *string         `json:"reason,omitempty"`
	CreatedBy       string          `json:"createdBy"`
	CreatedAt       time.Time       `format:"date-time" json:"createdAt"`
	UpdatedAt       *time.Time      `format:"date-time" json:"updatedAt,omitempty"`
}

type CreateCustomPolicyRequest struct {
	PolicyTypeID    int64            `binding:"required,gt=0" json:"policyTypeId"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Direction       Direction        `binding:"omitempty,oneof=ALL OUT IN" enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType       `binding:"omitempty,oneof=NONE DAILY MONTHLY LIFETIME" enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	Operator        Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value           *decimal.Decimal `binding:"required" example:"10000" json:"value" swaggertype:"string"`
	EffectiveFrom   *time.Time       `format:"date-time" json:"effectiveFrom"`
	EffectiveTo     *time.Time       `format:"date-time" json:"effectiveTo"`
	Reason          *string          `json:"reason"`
	UpdatedBy       string           `binding:"required" json:"updatedBy"`
}

type UpdateCustomPolicyRequest struct {
	Operator      Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value         *decimal.Decimal `binding:"required" example:"10000" json:"value" swaggertype:"string"`
	EffectiveFrom *time.Time       `binding:"required" format:"date-time" json:"effectiveFrom"`
	EffectiveTo   *time.Time       `format:"date-time" json:"effectiveTo"`
	Reason        *string          `json:"reason"`
	UpdatedBy     string           `binding:"required" json:"updatedBy"`
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
	PolicyType      string          `json:"policyType"`
	Unit            string          `json:"unit"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Direction       Direction       `enums:"ALL,OUT,IN" json:"direction"`
	WindowType      WindowType      `enums:"NONE,DAILY,MONTHLY,LIFETIME" json:"windowType"`
	Operator        Operator        `enums:"EQ,NEQ,GT,GTE,LT,LTE" json:"operator"`
	Value           decimal.Decimal `example:"10000" json:"value" swaggertype:"string"`
	Source          Source          `enums:"SYSTEM,USER" json:"source"`
}
