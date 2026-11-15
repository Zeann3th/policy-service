package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	db "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

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

func (s *limitStoreImpl) transact(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = fn(db.New(tx)); err != nil {
		return mapDBError(err)
	}
	return tx.Commit(ctx)
}

func audit(ctx context.Context, q *db.Queries, entity, key string, action limit.AuditAction, oldValue, newValue any, actor string) error {
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
	_, err = q.CreateLimitAuditLog(ctx, db.CreateLimitAuditLogParams{Entity: entity, EntityKey: key, Action: string(action), OldValue: oldJSON, NewValue: newJSON, UpdatedBy: actor})
	return err
}

func toUnit(v db.Unit) limit.Unit {
	return limit.Unit{ID: v.ID, Code: v.Code, Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toType(v db.LimitType) limit.LimitType {
	return limit.LimitType{ID: v.ID, Code: v.Code, Description: v.Description, UnitID: v.UnitID, Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toGroup(v db.LimitGroup) limit.LimitGroup {
	return limit.LimitGroup{ID: v.ID, WalletType: v.WalletType, Currency: v.Currency, Tier: v.Tier, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toValue(v db.LimitValue) limit.LimitValue {
	return limit.LimitValue{ID: v.ID, LimitGroupID: v.LimitGroupID, LimitTypeID: v.LimitTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Operator: limit.Operator(v.Operator), Value: v.Value, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
func toUser(v db.UserLimit) limit.UserLimit {
	return limit.UserLimit{ID: v.ID, WalletID: v.WalletID, LimitTypeID: v.LimitTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Operator: limit.Operator(v.Operator), Value: v.Value, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *limitStoreImpl) CreateUnit(ctx context.Context, c limit.CreateUnitRequest) (out limit.Unit, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		v, e := q.CreateUnit(ctx, db.CreateUnitParams{Code: c.Code, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toUnit(v)
		return audit(ctx, q, entityUnit, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetUnit(ctx context.Context, id int64) (limit.Unit, error) {
	v, e := s.admin.GetUnitByID(ctx, id)
	return toUnit(v), mapDBError(e)
}
func (s *limitStoreImpl) ListUnits(ctx context.Context, l, o int32) ([]limit.Unit, int64, error) {
	vs, e := s.admin.ListUnits(ctx, db.ListUnitsParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.admin.CountUnits(ctx)
	out := make([]limit.Unit, len(vs))
	for i, v := range vs {
		out[i] = toUnit(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateUnit(ctx context.Context, id int64, active bool, actor string) (out limit.Unit, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		old, e := q.GetUnitByIDForUpdate(ctx, id)
		if e != nil {
			return e
		}
		v, e := q.UpdateUnit(ctx, db.UpdateUnitParams{ID: id, Active: active})
		if e != nil {
			return e
		}
		out = toUnit(v)
		return audit(ctx, q, entityUnit, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, actor)
	})
	return
}
func (s *limitStoreImpl) DeleteUnit(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *db.Queries) error {
		v, e := q.DeleteUnit(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityUnit, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateLimitType(ctx context.Context, c limit.CreateLimitTypeRequest) (out limit.LimitType, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		v, e := q.CreateLimitType(ctx, db.CreateLimitTypeParams{Code: c.Code, Description: c.Description, UnitID: c.UnitID, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toType(v)
		return audit(ctx, q, entityLimitType, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetLimitType(ctx context.Context, id int64) (limit.LimitType, error) {
	v, e := s.admin.GetLimitTypeByID(ctx, id)
	return toType(v), mapDBError(e)
}
func (s *limitStoreImpl) ListLimitTypes(ctx context.Context, l, o int32) ([]limit.LimitType, int64, error) {
	vs, e := s.admin.ListLimitTypes(ctx, db.ListLimitTypesParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.admin.CountLimitTypes(ctx)
	out := make([]limit.LimitType, len(vs))
	for i, v := range vs {
		out[i] = toType(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateLimitType(ctx context.Context, id int64, c limit.UpdateLimitTypeRequest) (out limit.LimitType, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		old, e := q.GetLimitTypeByIDForUpdate(ctx, id)
		if e != nil {
			return e
		}
		v, e := q.UpdateLimitType(ctx, db.UpdateLimitTypeParams{ID: id, Description: c.Description, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toType(v)
		return audit(ctx, q, entityLimitType, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) DeleteLimitType(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *db.Queries) error {
		v, e := q.DeleteLimitType(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityLimitType, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateLimitGroup(ctx context.Context, c limit.CreateLimitGroupRequest) (out limit.LimitGroup, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		v, e := q.CreateLimitGroup(ctx, db.CreateLimitGroupParams{WalletType: c.WalletType, Currency: c.Currency, Tier: c.Tier})
		if e != nil {
			return e
		}
		out = toGroup(v)
		return audit(ctx, q, entityLimitGroup, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetLimitGroup(ctx context.Context, id int64) (limit.LimitGroup, error) {
	v, e := s.admin.GetLimitGroupByID(ctx, id)
	return toGroup(v), mapDBError(e)
}
func (s *limitStoreImpl) DeleteLimitGroup(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *db.Queries) error {
		v, e := q.DeleteLimitGroup(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityLimitGroup, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateLimitValue(ctx context.Context, g int64, c limit.CreateLimitValueRequest) (out limit.LimitValue, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		v, e := q.CreateLimitValue(ctx, db.CreateLimitValueParams{LimitGroupID: g, LimitTypeID: c.LimitTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Operator: string(c.Operator), Value: *c.Value})
		if e != nil {
			return e
		}
		out = toValue(v)
		return audit(ctx, q, entityLimitValue, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetLimitValue(ctx context.Context, g, id int64) (limit.LimitValue, error) {
	v, e := s.admin.GetLimitValueByID(ctx, db.GetLimitValueByIDParams{LimitGroupID: g, ID: id})
	return toValue(v), mapDBError(e)
}
func (s *limitStoreImpl) ListLimitValues(ctx context.Context, g int64, l, o int32) ([]limit.LimitValue, int64, error) {
	vs, e := s.admin.ListLimitValues(ctx, db.ListLimitValuesParams{LimitGroupID: g, PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.admin.CountLimitValues(ctx, g)
	out := make([]limit.LimitValue, len(vs))
	for i, v := range vs {
		out[i] = toValue(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateLimitValue(ctx context.Context, g, id int64, c limit.UpdateLimitValueRequest) (out limit.LimitValue, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		old, e := q.GetLimitValueByIDForUpdate(ctx, db.GetLimitValueByIDForUpdateParams{LimitGroupID: g, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdateLimitValue(ctx, db.UpdateLimitValueParams{LimitGroupID: g, ID: id, Operator: string(c.Operator), Value: *c.Value})
		if e != nil {
			return e
		}
		out = toValue(v)
		return audit(ctx, q, entityLimitValue, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) DeleteLimitValue(ctx context.Context, g, id int64, actor string) error {
	return s.transact(ctx, func(q *db.Queries) error {
		v, e := q.DeleteLimitValue(ctx, db.DeleteLimitValueParams{LimitGroupID: g, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityLimitValue, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

func (s *limitStoreImpl) CreateUserLimit(ctx context.Context, w string, c limit.CreateUserLimitRequest) (out limit.UserLimit, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		v, e := q.CreateUserLimit(ctx, db.CreateUserLimitParams{WalletID: w, LimitTypeID: c.LimitTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Operator: string(c.Operator), Value: *c.Value, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason, CreatedBy: c.UpdatedBy})
		if e != nil {
			return e
		}
		out = toUser(v)
		return audit(ctx, q, entityUserLimit, strconv.FormatInt(v.ID, 10), limit.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) GetUserLimit(ctx context.Context, w string, id int64) (limit.UserLimit, error) {
	v, e := s.admin.GetUserLimitByID(ctx, db.GetUserLimitByIDParams{WalletID: w, ID: id})
	return toUser(v), mapDBError(e)
}
func (s *limitStoreImpl) ListUserLimits(ctx context.Context, w string, l, o int32) ([]limit.UserLimit, int64, error) {
	vs, e := s.admin.ListUserLimits(ctx, db.ListUserLimitsParams{WalletID: w, PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.admin.CountUserLimits(ctx, w)
	out := make([]limit.UserLimit, len(vs))
	for i, v := range vs {
		out[i] = toUser(v)
	}
	return out, total, e
}
func (s *limitStoreImpl) UpdateUserLimit(ctx context.Context, w string, id int64, c limit.UpdateUserLimitRequest) (out limit.UserLimit, err error) {
	err = s.transact(ctx, func(q *db.Queries) error {
		old, e := q.GetUserLimitByIDForUpdate(ctx, db.GetUserLimitByIDForUpdateParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdateUserLimit(ctx, db.UpdateUserLimitParams{WalletID: w, ID: id, Operator: string(c.Operator), Value: *c.Value, EffectiveFrom: *c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason})
		if e != nil {
			return e
		}
		out = toUser(v)
		return audit(ctx, q, entityUserLimit, strconv.FormatInt(id, 10), limit.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}
func (s *limitStoreImpl) DeleteUserLimit(ctx context.Context, w string, id int64, actor string) error {
	return s.transact(ctx, func(q *db.Queries) error {
		v, e := q.DeleteUserLimit(ctx, db.DeleteUserLimitParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityUserLimit, strconv.FormatInt(id, 10), limit.AuditDelete, v, nil, actor)
	})
}

var _ limit.LimitAdminStore = (*limitStoreImpl)(nil)
