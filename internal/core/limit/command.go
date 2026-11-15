package limit

import (
	"time"

	"github.com/shopspring/decimal"
)

const MaxPage int32 = 21474836

type PageRequest struct {
	Page  int32 `binding:"omitempty,min=1,max=21474836" form:"page" json:"page"`
	Limit int32 `binding:"omitempty,min=1,max=100" form:"limit" json:"limit"`
}

func (p PageRequest) Values() (page, limit, offset int32) {
	page, limit = p.Page, p.Limit
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 20
	}
	return page, limit, (page - 1) * limit
}

type Page[T any] struct {
	Items []T   `json:"items"`
	Page  int32 `json:"page"`
	Limit int32 `json:"limit"`
	Total int64 `json:"total"`
}

type ActorRequest struct {
	UpdatedBy string `binding:"required" json:"updatedBy"`
}

type UnitResponse struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}
type CreateUnitRequest struct {
	Code      string `binding:"required" json:"code"`
	Active    *bool  `json:"active"`
	UpdatedBy string `binding:"required" json:"updatedBy"`
}
type UpdateUnitRequest struct {
	Active    *bool  `binding:"required" json:"active"`
	UpdatedBy string `binding:"required" json:"updatedBy"`
}

type LimitTypeResponse struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Description *string    `json:"description,omitempty"`
	UnitID      int64      `json:"unitId"`
	Active      bool       `json:"active"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   *time.Time `json:"updatedAt,omitempty"`
}
type CreateLimitTypeRequest struct {
	Code        string  `binding:"required" json:"code"`
	Description *string `json:"description"`
	UnitID      int64   `binding:"required,gt=0" json:"unitId"`
	Active      *bool   `json:"active"`
	UpdatedBy   string  `binding:"required" json:"updatedBy"`
}
type UpdateLimitTypeRequest struct {
	Description *string `json:"description"`
	Active      *bool   `binding:"required" json:"active"`
	UpdatedBy   string  `binding:"required" json:"updatedBy"`
}

type LimitGroupResponse struct {
	ID         int64      `json:"id"`
	WalletType string     `json:"walletType"`
	Currency   string     `json:"currency"`
	Tier       string     `json:"tier"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  *time.Time `json:"updatedAt,omitempty"`
}
type CreateLimitGroupRequest struct {
	WalletType string `binding:"required" json:"walletType"`
	Currency   string `binding:"required,iso4217" json:"currency"`
	Tier       string `binding:"required" json:"tier"`
	UpdatedBy  string `binding:"required" json:"updatedBy"`
}

type LimitValueResponse struct {
	ID              int64           `json:"id"`
	LimitGroupID    int64           `json:"limitGroupId"`
	LimitTypeID     int64           `json:"limitTypeId"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Operator        Operator        `json:"operator"`
	Value           decimal.Decimal `json:"value"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       *time.Time      `json:"updatedAt,omitempty"`
}
type CreateLimitValueRequest struct {
	LimitTypeID     int64            `binding:"required,gt=0" json:"limitTypeId"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Operator        Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" json:"operator"`
	Value           *decimal.Decimal `binding:"required" json:"value"`
	UpdatedBy       string           `binding:"required" json:"updatedBy"`
}
type UpdateLimitValueRequest struct {
	Operator  Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" json:"operator"`
	Value     *decimal.Decimal `binding:"required" json:"value"`
	UpdatedBy string           `binding:"required" json:"updatedBy"`
}

type UserLimitResponse struct {
	ID              int64           `json:"id"`
	WalletID        string          `json:"walletId"`
	LimitTypeID     int64           `json:"limitTypeId"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Operator        Operator        `json:"operator"`
	Value           decimal.Decimal `json:"value"`
	EffectiveFrom   time.Time       `json:"effectiveFrom"`
	EffectiveTo     *time.Time      `json:"effectiveTo,omitempty"`
	Reason          *string         `json:"reason,omitempty"`
	CreatedBy       string          `json:"createdBy"`
	CreatedAt       time.Time       `json:"createdAt"`
	UpdatedAt       *time.Time      `json:"updatedAt,omitempty"`
}
type CreateUserLimitRequest struct {
	LimitTypeID     int64            `binding:"required,gt=0" json:"limitTypeId"`
	TransactionType string           `json:"transactionType"`
	Channel         string           `json:"channel"`
	Operator        Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" json:"operator"`
	Value           *decimal.Decimal `binding:"required" json:"value"`
	EffectiveFrom   *time.Time       `json:"effectiveFrom"`
	EffectiveTo     *time.Time       `json:"effectiveTo"`
	Reason          *string          `json:"reason"`
	UpdatedBy       string           `binding:"required" json:"updatedBy"`
}
type UpdateUserLimitRequest struct {
	Operator      Operator         `binding:"required,oneof=EQ NEQ GT GTE LT LTE" json:"operator"`
	Value         *decimal.Decimal `binding:"required" json:"value"`
	EffectiveFrom *time.Time       `binding:"required" json:"effectiveFrom"`
	EffectiveTo   *time.Time       `json:"effectiveTo"`
	Reason        *string          `json:"reason"`
	UpdatedBy     string           `binding:"required" json:"updatedBy"`
}

type GetLimitConfigRequest struct {
	WalletType      string `binding:"required" form:"walletType" json:"walletType"`
	Currency        string `binding:"required,iso4217" form:"currency" json:"currency"`
	Tier            string `binding:"required" form:"tier" json:"tier"`
	WalletID        string `binding:"required" form:"walletId" json:"walletId"`
	TransactionType string `form:"transactionType" json:"transactionType,omitempty"`
}

type GetLimitConfigResponse struct {
	Rules []LimitRule `json:"rules"`
}

type LimitRule struct {
	LimitType       string          `json:"limitType"`
	Unit            string          `json:"unit"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Operator        Operator        `json:"operator"`
	Value           decimal.Decimal `json:"value"`
	Source          Source          `json:"source"`
}
