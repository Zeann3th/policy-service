package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	sqlcdb "github.com/zeann3th/policy-service/internal/infrastructure/postgres/sqlcdb"
)

type limitStoreImpl struct {
	queries sqlcdb.Querier
}

func NewLimitStore(db *pgxpool.Pool) limit.LimitStore {
	return &limitStoreImpl{
		queries: sqlcdb.New(db),
	}
}

func (s *limitStoreImpl) GetLimitGroupID(
	ctx context.Context,
	walletType string,
	currency string,
	tier string,
) (int64, error) {
	groupID, err := s.queries.GetLimitGroupID(ctx, sqlcdb.GetLimitGroupIDParams{
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
			WalletID:        row.WalletID,
			LimitTypeID:     row.LimitTypeID,
			LimitType:       row.LimitType,
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

func toLimitValues(rows []sqlcdb.GetLimitValuesByGroupIDRow) []limit.LimitValue {
	values := make([]limit.LimitValue, len(rows))
	for i, row := range rows {
		values[i] = limit.LimitValue{
			LimitGroupID:    row.LimitGroupID,
			LimitTypeID:     row.LimitTypeID,
			LimitType:       row.LimitType,
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
