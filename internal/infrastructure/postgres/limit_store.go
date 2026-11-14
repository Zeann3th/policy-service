package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/haphaestus/internal/core/limit"
	sqlcdb "github.com/zeann3th/haphaestus/internal/infrastructure/postgres/sqlcdb"
)

type limitStoreImpl struct {
	queries *sqlcdb.Queries
}

func NewLimitStore(db *pgxpool.Pool) limit.LimitStore {
	return &limitStoreImpl{
		queries: sqlcdb.New(db),
	}
}

func (s *limitStoreImpl) HasLimitTypes(ctx context.Context) (bool, error) {
	return s.queries.HasLimitTypes(ctx)
}

func (s *limitStoreImpl) GetLimitGroupID(
	ctx context.Context,
	walletTypeCode string,
	currencyCode string,
	tierCode string,
) (int64, error) {
	return s.queries.GetLimitGroupID(ctx, sqlcdb.GetLimitGroupIDParams{
		WalletTypeCode: walletTypeCode,
		CurrencyCode:   currencyCode,
		TierCode:       tierCode,
	})
}

func (s *limitStoreImpl) GetLimitValuesByGroupID(ctx context.Context, groupID int64) ([]limit.LimitValue, error) {
	rows, err := s.queries.GetLimitValuesByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return toLimitValues(rows), nil
}

func toLimitValues(rows []sqlcdb.LimitValue) []limit.LimitValue {
	values := make([]limit.LimitValue, len(rows))
	for i, row := range rows {
		values[i] = limit.LimitValue{
			LimitGroupID:        row.LimitGroupID,
			LimitTypeCode:       row.LimitTypeCode,
			TransactionTypeCode: row.TransactionTypeCode,
			ChannelCode:         row.ChannelCode,
			Operator:            limit.Operator(row.Operator),
			Value:               row.Value,
			CreatedAt:           row.CreatedAt,
			UpdatedAt:           row.UpdatedAt,
		}
	}
	return values
}
