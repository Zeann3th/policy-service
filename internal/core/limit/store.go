package limit

import "context"

type LimitStore interface {
	GetLimitGroupID(ctx context.Context, walletType, currency, tier string) (int64, error)
	GetLimitValuesByGroupID(ctx context.Context, groupID int64) ([]LimitValue, error)
	GetActiveUserLimits(ctx context.Context, walletID string) ([]UserLimit, error)
}
