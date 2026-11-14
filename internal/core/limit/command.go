package limit

import "github.com/shopspring/decimal"

type GetLimitConfigRequest struct {
	WalletType string `binding:"required" form:"walletType" json:"walletType"`
	Currency   string `binding:"required" form:"currency" json:"currency"`
	Tier       string `binding:"required" form:"tier" json:"tier"`
	WalletID   string `binding:"required" form:"walletId" json:"walletId"`
}

type GetLimitConfigResponse struct {
	Rules []LimitRule `json:"rules"`
}

type LimitRule struct {
	LimitType       string          `json:"limitType"`
	TransactionType string          `json:"transactionType"`
	Channel         string          `json:"channel"`
	Operator        Operator        `json:"operator"`
	Value           decimal.Decimal `json:"value"`
}
