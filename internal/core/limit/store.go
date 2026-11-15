package limit

import "context"

type LimitStore interface {
	GetUnits(ctx context.Context) ([]Unit, error)
	GetLimitGroupID(ctx context.Context, walletType, currency, tier string) (int64, error)
	GetLimitValuesByGroupID(ctx context.Context, groupID int64) ([]LimitValue, error)
	GetActiveUserLimits(ctx context.Context, walletID string) ([]UserLimit, error)
}

type LimitAdminStore interface {
	CreateUnit(context.Context, CreateUnitRequest) (Unit, error)
	GetUnit(context.Context, int64) (Unit, error)
	ListUnits(context.Context, int32, int32) ([]Unit, int64, error)
	UpdateUnit(context.Context, int64, bool, string) (Unit, error)
	DeleteUnit(context.Context, int64, string) error
	CreateLimitType(context.Context, CreateLimitTypeRequest) (LimitType, error)
	GetLimitType(context.Context, int64) (LimitType, error)
	ListLimitTypes(context.Context, int32, int32) ([]LimitType, int64, error)
	UpdateLimitType(context.Context, int64, UpdateLimitTypeRequest) (LimitType, error)
	DeleteLimitType(context.Context, int64, string) error
	CreateLimitGroup(context.Context, CreateLimitGroupRequest) (LimitGroup, error)
	GetLimitGroup(context.Context, int64) (LimitGroup, error)
	DeleteLimitGroup(context.Context, int64, string) error
	CreateLimitValue(context.Context, int64, CreateLimitValueRequest) (LimitValue, error)
	GetLimitValue(context.Context, int64, int64) (LimitValue, error)
	ListLimitValues(context.Context, int64, int32, int32) ([]LimitValue, int64, error)
	UpdateLimitValue(context.Context, int64, int64, UpdateLimitValueRequest) (LimitValue, error)
	DeleteLimitValue(context.Context, int64, int64, string) error
	CreateUserLimit(context.Context, string, CreateUserLimitRequest) (UserLimit, error)
	GetUserLimit(context.Context, string, int64) (UserLimit, error)
	ListUserLimits(context.Context, string, int32, int32) ([]UserLimit, int64, error)
	UpdateUserLimit(context.Context, string, int64, UpdateUserLimitRequest) (UserLimit, error)
	DeleteUserLimit(context.Context, string, int64, string) error
}
