package limit

import (
	"context"
	"fmt"
	"log/slog"
)

type GetLimitCommand struct {
	WalletTypeCode string
	CurrencyCode   string
	TierCode       string
	WalletID       string
}

type LimitService interface {
	GetLimitConfig(context.Context, GetLimitCommand) ([]LimitValue, error)
}

func (s *limitServiceImpl) GetLimitConfig(ctx context.Context, command GetLimitCommand) ([]LimitValue, error) {
	groupID, err := s.store.GetLimitGroupID(
		ctx,
		command.WalletTypeCode,
		command.CurrencyCode,
		command.TierCode,
	)
	if err != nil {
		return nil, fmt.Errorf("get limit group: %w", err)
	}

	values, err := s.store.GetLimitValuesByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get limit values: %w", err)
	}
	return values, nil
}

type limitServiceImpl struct {
	logger *slog.Logger
	store  LimitStore
}

func NewService(
	logger *slog.Logger,
	store LimitStore,
) LimitService {
	return &limitServiceImpl{
		logger: logger,
		store:  store,
	}
}
