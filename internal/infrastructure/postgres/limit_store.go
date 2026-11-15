package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

type limitStoreImpl struct {
	db              *pgxpool.Pool
	resolverQueries limitQueries
	queries         *codegen.Queries
}

type limitQueries interface {
	GetUnits(context.Context) ([]codegen.Unit, error)
	GetLimitGroupID(context.Context, codegen.GetLimitGroupIDParams) (int64, error)
	GetLimitValuesByGroupID(context.Context, int64) ([]codegen.GetLimitValuesByGroupIDRow, error)
	GetActiveUserLimits(context.Context, string) ([]codegen.GetActiveUserLimitsRow, error)
}

func NewLimitStore(db *pgxpool.Pool) limit.LimitStore {
	return &limitStoreImpl{
		db:              db,
		resolverQueries: codegen.New(db),
		queries:         codegen.New(db),
	}
}

func (s *limitStoreImpl) GetUnits(ctx context.Context) ([]limit.Unit, error) {
	rows, err := s.resolverQueries.GetUnits(ctx)
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
	groupID, err := s.resolverQueries.GetLimitGroupID(ctx, codegen.GetLimitGroupIDParams{
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
	rows, err := s.resolverQueries.GetActiveUserLimits(ctx, walletID)
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
	rows, err := s.resolverQueries.GetLimitValuesByGroupID(ctx, groupID)
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

const (
	entityUnit       = "unit"
	entityLimitType  = "limit_type"
	entityLimitGroup = "limit_group"
	entityLimitValue = "limit_value"
	entityUserLimit  = "user_limit"
)

func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return core.ErrNotFound.WithCause(err)
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return core.ErrConflict.WithCause(err)
		case "23503":
			return core.ErrNotFound.WithCause(err)
		case "23514":
			return core.ErrBadRequest.WithCause(err)
		}
	}
	return err
}

func (s *limitStoreImpl) transact(ctx context.Context, fn func(*codegen.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(codegen.New(tx)); err != nil {
		return mapDBError(err)
	}
	return tx.Commit(ctx)
}

func audit(ctx context.Context, q *codegen.Queries, entity, key string, action limit.AuditAction, oldValue, newValue any, actor string) error {
	var oldJSON, newJSON []byte
	var err error
	if oldValue != nil {
		oldJSON, err = json.Marshal(oldValue)
		if err != nil {
			return err
		}
	}
	if newValue != nil {
		newJSON, err = json.Marshal(newValue)
		if err != nil {
			return err
		}
	}
	_, err = q.CreateLimitAuditLog(ctx, codegen.CreateLimitAuditLogParams{Entity: entity, EntityKey: key, Action: string(action), OldValue: oldJSON, NewValue: newJSON, UpdatedBy: actor})
	return err
}

func toUnit(v codegen.Unit) limit.Unit {
	return limit.Unit{ID: v.ID, Code: v.Code, Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toType(v codegen.LimitType) limit.LimitType {
	return limit.LimitType{ID: v.ID, Code: v.Code, Description: v.Description, UnitID: v.UnitID, Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toGroup(v codegen.LimitGroup) limit.LimitGroup {
	return limit.LimitGroup{ID: v.ID, WalletType: v.WalletType, Currency: v.Currency, Tier: v.Tier, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toValue(v codegen.LimitValue) limit.LimitValue {
	return limit.LimitValue{ID: v.ID, LimitGroupID: v.LimitGroupID, LimitTypeID: v.LimitTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Operator: limit.Operator(v.Operator), Value: v.Value, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toUser(v codegen.UserLimit) limit.UserLimit {
	return limit.UserLimit{ID: v.ID, WalletID: v.WalletID, LimitTypeID: v.LimitTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Operator: limit.Operator(v.Operator), Value: v.Value, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *limitStoreImpl) CreateUnit(ctx context.Context, c limit.CreateUnitRequest) (out limit.Unit, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreateUnit(ctx, codegen.CreateUnitParams{Code: c.Code, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toUnit(v)
		return audit(ctx, q, entityUnit, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetUnit(ctx context.Context, id int64) (limit.Unit, error) {
	v, e := s.queries.GetUnitByID(ctx, id)
	return toUnit(v), mapDBError(e)
}
func (s *limitStoreImpl) ListUnits(ctx context.Context, l, o int32) ([]limit.Unit, int64, error) {
	vs, e := s.queries.ListUnits(ctx, codegen.ListUnitsParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountUnits(ctx)
	out := make([]limit.Unit, len(vs))
	for i, v := range vs {
		out[i] = toUnit(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateUnit(ctx context.Context, id int64, active bool, actor string) (out limit.Unit, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		old, e := q.GetUnitByIDForUpdate(ctx, id)
		if e != nil {
			return e
		}
		v, e := q.UpdateUnit(ctx, codegen.UpdateUnitParams{ID: id, Active: active})
		if e != nil {
			return e
		}
		out = toUnit(v)
		return audit(ctx, q, entityUnit, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, actor)
	})
	return
}
func (s *limitStoreImpl) DeleteUnit(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeleteUnit(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityUnit, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateLimitType(ctx context.Context, c limit.CreateLimitTypeRequest) (out limit.LimitType, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreateLimitType(ctx, codegen.CreateLimitTypeParams{Code: c.Code, Description: c.Description, UnitID: c.UnitID, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toType(v)
		return audit(ctx, q, entityLimitType, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetLimitType(ctx context.Context, id int64) (limit.LimitType, error) {
	v, e := s.queries.GetLimitTypeByID(ctx, id)
	return toType(v), mapDBError(e)
}
func (s *limitStoreImpl) ListLimitTypes(ctx context.Context, l, o int32) ([]limit.LimitType, int64, error) {
	vs, e := s.queries.ListLimitTypes(ctx, codegen.ListLimitTypesParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountLimitTypes(ctx)
	out := make([]limit.LimitType, len(vs))
	for i, v := range vs {
		out[i] = toType(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateLimitType(ctx context.Context, id int64, c limit.UpdateLimitTypeRequest) (out limit.LimitType, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		old, e := q.GetLimitTypeByIDForUpdate(ctx, id)
		if e != nil {
			return e
		}
		v, e := q.UpdateLimitType(ctx, codegen.UpdateLimitTypeParams{ID: id, Description: c.Description, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toType(v)
		return audit(ctx, q, entityLimitType, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) DeleteLimitType(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeleteLimitType(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityLimitType, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateLimitGroup(ctx context.Context, c limit.CreateLimitGroupRequest) (out limit.LimitGroup, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreateLimitGroup(ctx, codegen.CreateLimitGroupParams{WalletType: c.WalletType, Currency: c.Currency, Tier: c.Tier})
		if e != nil {
			return e
		}
		out = toGroup(v)
		return audit(ctx, q, entityLimitGroup, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetLimitGroup(ctx context.Context, id int64) (limit.LimitGroup, error) {
	v, e := s.queries.GetLimitGroupByID(ctx, id)
	return toGroup(v), mapDBError(e)
}
func (s *limitStoreImpl) DeleteLimitGroup(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeleteLimitGroup(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityLimitGroup, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateLimitValue(ctx context.Context, g int64, c limit.CreateLimitValueRequest) (out limit.LimitValue, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreateLimitValue(ctx, codegen.CreateLimitValueParams{LimitGroupID: g, LimitTypeID: c.LimitTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Operator: string(c.Operator), Value: *c.Value})
		if e != nil {
			return e
		}
		out = toValue(v)
		return audit(ctx, q, entityLimitValue, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetLimitValue(ctx context.Context, g, id int64) (limit.LimitValue, error) {
	v, e := s.queries.GetLimitValueByID(ctx, codegen.GetLimitValueByIDParams{LimitGroupID: g, ID: id})
	return toValue(v), mapDBError(e)
}
func (s *limitStoreImpl) ListLimitValues(ctx context.Context, g int64, l, o int32) ([]limit.LimitValue, int64, error) {
	vs, e := s.queries.ListLimitValues(ctx, codegen.ListLimitValuesParams{LimitGroupID: g, PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountLimitValues(ctx, g)
	out := make([]limit.LimitValue, len(vs))
	for i, v := range vs {
		out[i] = toValue(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateLimitValue(ctx context.Context, g, id int64, c limit.UpdateLimitValueRequest) (out limit.LimitValue, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		old, e := q.GetLimitValueByIDForUpdate(ctx, codegen.GetLimitValueByIDForUpdateParams{LimitGroupID: g, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdateLimitValue(ctx, codegen.UpdateLimitValueParams{LimitGroupID: g, ID: id, Operator: string(c.Operator), Value: *c.Value})
		if e != nil {
			return e
		}
		out = toValue(v)
		return audit(ctx, q, entityLimitValue, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) DeleteLimitValue(ctx context.Context, g, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeleteLimitValue(ctx, codegen.DeleteLimitValueParams{LimitGroupID: g, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityLimitValue, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateUserLimit(ctx context.Context, w string, c limit.CreateUserLimitRequest) (out limit.UserLimit, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreateUserLimit(ctx, codegen.CreateUserLimitParams{WalletID: w, LimitTypeID: c.LimitTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Operator: string(c.Operator), Value: *c.Value, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason, CreatedBy: c.UpdatedBy})
		if e != nil {
			return e
		}
		out = toUser(v)
		return audit(ctx, q, entityUserLimit, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetUserLimit(ctx context.Context, w string, id int64) (limit.UserLimit, error) {
	v, e := s.queries.GetUserLimitByID(ctx, codegen.GetUserLimitByIDParams{WalletID: w, ID: id})
	return toUser(v), mapDBError(e)
}
func (s *limitStoreImpl) ListUserLimits(ctx context.Context, w string, l, o int32) ([]limit.UserLimit, int64, error) {
	vs, e := s.queries.ListUserLimits(ctx, codegen.ListUserLimitsParams{WalletID: w, PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountUserLimits(ctx, w)
	out := make([]limit.UserLimit, len(vs))
	for i, v := range vs {
		out[i] = toUser(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateUserLimit(ctx context.Context, w string, id int64, c limit.UpdateUserLimitRequest) (out limit.UserLimit, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		old, e := q.GetUserLimitByIDForUpdate(ctx, codegen.GetUserLimitByIDForUpdateParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdateUserLimit(ctx, codegen.UpdateUserLimitParams{WalletID: w, ID: id, Operator: string(c.Operator), Value: *c.Value, EffectiveFrom: *c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason})
		if e != nil {
			return e
		}
		out = toUser(v)
		return audit(ctx, q, entityUserLimit, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) DeleteUserLimit(ctx context.Context, w string, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeleteUserLimit(ctx, codegen.DeleteUserLimitParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityUserLimit, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

var _ limit.LimitStore = (*limitStoreImpl)(nil)
