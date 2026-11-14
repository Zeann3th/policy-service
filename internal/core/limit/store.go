package limit

import "context"

type LimitStore interface {
	HasLimitTypes(context.Context) (bool, error)
	GetLimitGroupID(context.Context, string, string, string) (int64, error)
	GetLimitValuesByGroupID(context.Context, int64) ([]LimitValue, error)
}
