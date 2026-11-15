package limit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

const unitsCacheKeyPart = "units"

type LimitService interface {
	GetLimitConfig(context.Context, GetLimitConfigRequest) (GetLimitConfigResponse, error)
}

type LimitAdminService interface {
	CreateUnit(context.Context, CreateUnitRequest) (UnitResponse, error)
	GetUnit(context.Context, int64) (UnitResponse, error)
	ListUnits(context.Context, PageRequest) (Page[UnitResponse], error)
	UpdateUnit(context.Context, int64, UpdateUnitRequest) (UnitResponse, error)
	DeleteUnit(context.Context, int64, ActorRequest) error
	CreateLimitType(context.Context, CreateLimitTypeRequest) (LimitTypeResponse, error)
	GetLimitType(context.Context, int64) (LimitTypeResponse, error)
	ListLimitTypes(context.Context, PageRequest) (Page[LimitTypeResponse], error)
	UpdateLimitType(context.Context, int64, UpdateLimitTypeRequest) (LimitTypeResponse, error)
	DeleteLimitType(context.Context, int64, ActorRequest) error
	CreateLimitGroup(context.Context, CreateLimitGroupRequest) (LimitGroupResponse, error)
	GetLimitGroup(context.Context, int64) (LimitGroupResponse, error)
	DeleteLimitGroup(context.Context, int64, ActorRequest) error
	CreateLimitValue(context.Context, int64, CreateLimitValueRequest) (LimitValueResponse, error)
	GetLimitValue(context.Context, int64, int64) (LimitValueResponse, error)
	ListLimitValues(context.Context, int64, PageRequest) (Page[LimitValueResponse], error)
	UpdateLimitValue(context.Context, int64, int64, UpdateLimitValueRequest) (LimitValueResponse, error)
	DeleteLimitValue(context.Context, int64, int64, ActorRequest) error
	CreateUserLimit(context.Context, string, CreateUserLimitRequest) (UserLimitResponse, error)
	GetUserLimit(context.Context, string, int64) (UserLimitResponse, error)
	ListUserLimits(context.Context, string, PageRequest) (Page[UserLimitResponse], error)
	UpdateUserLimit(context.Context, string, int64, UpdateUserLimitRequest) (UserLimitResponse, error)
	DeleteUserLimit(context.Context, string, int64, ActorRequest) error
}

type Service interface {
	LimitService
	LimitAdminService
}

type limitServiceImpl struct {
	logger      *slog.Logger
	store       LimitStore
	cache       cacheport.Cache
	cachePrefix string
}

func NewService(
	logger *slog.Logger,
	store LimitStore,
	cache cacheport.Cache,
	serviceName string,
) Service {
	return &limitServiceImpl{
		logger:      logger,
		store:       store,
		cache:       cache,
		cachePrefix: core.Join(serviceName, "limit"),
	}
}

func (s *limitServiceImpl) GetLimitConfig(ctx context.Context, command GetLimitConfigRequest) (GetLimitConfigResponse, error) {
	values, err := s.getSystemLimitConfigs(ctx, command)
	if err != nil {
		return GetLimitConfigResponse{}, err
	}
	userLimits, err := s.store.GetActiveUserLimits(ctx, command.WalletID)
	if err != nil {
		return GetLimitConfigResponse{}, fmt.Errorf("get active user limits: %w", err)
	}
	values = mergeLimitValues(values, userLimits)
	values = filterLimitValuesByTransactionType(values, command.TransactionType)

	response := GetLimitConfigResponse{Rules: make([]LimitRule, len(values))}
	if len(values) == 0 {
		return response, nil
	}
	units, err := s.getUnits(ctx)
	if err != nil {
		return GetLimitConfigResponse{}, err
	}
	for i, value := range values {
		unit, ok := units[value.UnitID]
		if !ok {
			return GetLimitConfigResponse{}, fmt.Errorf("unit %d not found", value.UnitID)
		}
		response.Rules[i] = LimitRule{
			LimitType:       value.LimitType,
			Unit:            unit,
			TransactionType: value.TransactionType,
			Channel:         value.Channel,
			Operator:        value.Operator,
			Value:           value.Value,
			Source:          value.Source,
		}
	}
	return response, nil
}

func filterLimitValuesByTransactionType(values []LimitValue, transactionType string) []LimitValue {
	if transactionType == "" {
		return values
	}
	filtered := make([]LimitValue, 0, len(values))
	for _, value := range values {
		if value.TransactionType == All || value.TransactionType == transactionType {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func mergeLimitValues(systemValues []LimitValue, userLimits []UserLimit) []LimitValue {
	values := append([]LimitValue(nil), systemValues...)
	indexes := make(map[LimitKey]int, len(values))
	for i, value := range values {
		values[i].Source = SourceSystem
		indexes[value.Key()] = i
	}
	for _, userLimit := range userLimits {
		value := LimitValue{
			LimitTypeID:     userLimit.LimitTypeID,
			LimitType:       userLimit.LimitType,
			UnitID:          userLimit.UnitID,
			TransactionType: userLimit.TransactionType,
			Channel:         userLimit.Channel,
			Operator:        userLimit.Operator,
			Value:           userLimit.Value,
			Source:          SourceUser,
		}
		if i, ok := indexes[userLimit.Key()]; ok {
			values[i] = value
			continue
		}
		indexes[userLimit.Key()] = len(values)
		values = append(values, value)
	}
	return values
}

func (s *limitServiceImpl) getUnits(ctx context.Context) (map[int64]string, error) {
	cacheKey := s.limitConfigKey(ctx, unitsCacheKeyPart)
	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		units, err := core.Decode[map[int64]string](cached)
		if err == nil {
			return *units, nil
		}
	}

	items, err := s.store.GetUnits(ctx)
	if err != nil {
		return nil, fmt.Errorf("get units: %w", err)
	}
	units := make(map[int64]string, len(items))
	for _, item := range items {
		units[item.ID] = item.Code
	}
	encoded, err := core.Encode(&units)
	if err != nil {
		s.logger.Warn("encode units for cache", "key", cacheKey, "error", err)
		return units, nil
	}
	if err := s.cache.SetWithTTL(ctx, cacheKey, encoded, adminCacheTTL); err != nil {
		s.logger.Warn("cache units", "key", cacheKey, "error", err)
	}
	return units, nil
}

func (s *limitServiceImpl) getSystemLimitConfigs(ctx context.Context, command GetLimitConfigRequest) ([]LimitValue, error) {
	cacheKey := s.limitConfigKey(ctx,
		command.WalletType,
		command.Currency,
		command.Tier,
	)

	if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
		results, err := core.Decode[[]LimitValue](cached)
		if err == nil {
			return *results, nil
		}
	}

	groupID, err := s.store.GetLimitGroupID(
		ctx,
		command.WalletType,
		command.Currency,
		command.Tier,
	)
	if err != nil {
		return nil, fmt.Errorf("get limit group: %w", err)
	}

	values, err := s.store.GetLimitValuesByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get limit values: %w", err)
	}

	encoded, err := core.Encode(&values)
	if err != nil {
		s.logger.Warn("encode limit config for cache", "key", cacheKey, "error", err)
		return values, nil
	}
	if err := s.cache.SetWithTTL(ctx, cacheKey, encoded, adminCacheTTL); err != nil {
		s.logger.Warn("cache limit config", "key", cacheKey, "error", err)
	}
	return values, nil
}
