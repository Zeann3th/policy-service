package limit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/zeann3th/policy-service/internal/core"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
)

type LimitService interface {
	GetLimitConfig(context.Context, GetLimitConfigRequest) (GetLimitConfigResponse, error)
}

type limitServiceImpl struct {
	logger      *slog.Logger
	store       LimitStore
	cache       cacheport.Cache
	serviceName string
}

func NewService(
	logger *slog.Logger,
	store LimitStore,
	cache cacheport.Cache,
	serviceName string,
) LimitService {
	return &limitServiceImpl{
		logger:      logger,
		store:       store,
		cache:       cache,
		serviceName: serviceName,
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

	response := GetLimitConfigResponse{Rules: make([]LimitRule, len(values))}
	for i, value := range values {
		response.Rules[i] = LimitRule{
			LimitType:       value.LimitType,
			TransactionType: value.TransactionType,
			Channel:         value.Channel,
			Operator:        value.Operator,
			Value:           value.Value,
		}
	}
	return response, nil
}

func mergeLimitValues(systemValues []LimitValue, userLimits []UserLimit) []LimitValue {
	values := append([]LimitValue(nil), systemValues...)
	indexes := make(map[LimitKey]int, len(values))
	for i, value := range values {
		indexes[value.Key()] = i
	}
	for _, userLimit := range userLimits {
		value := LimitValue{
			LimitTypeID:     userLimit.LimitTypeID,
			LimitType:       userLimit.LimitType,
			TransactionType: userLimit.TransactionType,
			Channel:         userLimit.Channel,
			Operator:        userLimit.Operator,
			Value:           userLimit.Value,
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

func (s *limitServiceImpl) getSystemLimitConfigs(ctx context.Context, command GetLimitConfigRequest) ([]LimitValue, error) {
	cacheKey := core.Join(
		s.serviceName,
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
	if err := s.cache.Set(ctx, cacheKey, encoded); err != nil {
		s.logger.Warn("cache limit config", "key", cacheKey, "error", err)
	}
	return values, nil
}
