package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

type limitStoreImpl struct {
	db      *pgxpool.Pool
	queries limitQueries
	admin   *codegen.Queries
}

type limitQueries interface {
	GetUnits(context.Context) ([]codegen.Unit, error)
	GetLimitGroupID(context.Context, codegen.GetLimitGroupIDParams) (int64, error)
	GetLimitValuesByGroupID(context.Context, int64) ([]codegen.GetLimitValuesByGroupIDRow, error)
	GetActiveUserLimits(context.Context, string) ([]codegen.GetActiveUserLimitsRow, error)
}

func NewLimitStore(db *pgxpool.Pool) limit.LimitStore {
	return &limitStoreImpl{
		db:      db,
		queries: codegen.New(db),
		admin:   codegen.New(db),
	}
}

func (s *limitStoreImpl) GetUnits(ctx context.Context) ([]limit.Unit, error) {
	rows, err := s.queries.GetUnits(ctx)
	if err != nil {
		return nil, err
	}
	units := make([]limit.Unit, len(rows))
	for i, row := range rows {
		units[i] = limit.Unit{
			ID:        row.ID,
			Code:      row.Code,
			Active:    row.Active,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
	}
	return units, nil
}

func (s *limitStoreImpl) GetLimitGroupID(
	ctx context.Context,
	walletType string,
	currency string,
	tier string,
) (int64, error) {
	groupID, err := s.queries.GetLimitGroupID(ctx, codegen.GetLimitGroupIDParams{
		WalletType: walletType,
		Currency:   currency,
		Tier:       tier,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, core.ErrNotFound.WithCause(err)
	}
	return groupID, err
}

func (s *limitStoreImpl) GetActiveUserLimits(ctx context.Context, walletID string) ([]limit.UserLimit, error) {
	rows, err := s.queries.GetActiveUserLimits(ctx, walletID)
	if err != nil {
		return nil, err
	}
	values := make([]limit.UserLimit, len(rows))
	for i, row := range rows {
		values[i] = limit.UserLimit{
			ID:              row.ID,
			WalletID:        row.WalletID,
			LimitTypeID:     row.LimitTypeID,
			LimitType:       row.LimitType,
			UnitID:          row.UnitID,
			TransactionType: row.TransactionType,
			Channel:         row.Channel,
			Operator:        limit.Operator(row.Operator),
			Value:           row.Value,
			EffectiveFrom:   row.EffectiveFrom,
			EffectiveTo:     row.EffectiveTo,
			Reason:          row.Reason,
			CreatedBy:       row.CreatedBy,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
	}
	return values, nil
}

func (s *limitStoreImpl) GetLimitValuesByGroupID(ctx context.Context, groupID int64) ([]limit.LimitValue, error) {
	rows, err := s.queries.GetLimitValuesByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return toLimitValues(rows), nil
}

func toLimitValues(rows []codegen.GetLimitValuesByGroupIDRow) []limit.LimitValue {
	values := make([]limit.LimitValue, len(rows))
	for i, row := range rows {
		values[i] = limit.LimitValue{
			ID:              row.ID,
			LimitGroupID:    row.LimitGroupID,
			LimitTypeID:     row.LimitTypeID,
			LimitType:       row.LimitType,
			UnitID:          row.UnitID,
			TransactionType: row.TransactionType,
			Channel:         row.Channel,
			Operator:        limit.Operator(row.Operator),
			Value:           row.Value,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
	}
	return values
}
