package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// UnitStore implements unit.Store: the units catalog.
type UnitStore struct {
	db      *pgxpool.Pool
	queries *codegen.Queries
}

func NewUnitStore(db *pgxpool.Pool) *UnitStore {
	return &UnitStore{db: db, queries: codegen.New(db)}
}

func toUnit(v codegen.Unit) unit.Unit {
	return unit.Unit{ID: v.ID, Code: v.Code, Description: v.Description, Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *UnitStore) Create(ctx context.Context, c unit.CreateRequest) (out unit.Unit, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.CreateUnit(ctx, codegen.CreateUnitParams{Code: c.Code, Description: c.Description, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toUnit(v)
		return audit(ctx, q, entityUnit, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *UnitStore) Get(ctx context.Context, id int64) (unit.Unit, error) {
	v, e := s.queries.GetUnitByID(ctx, id)
	return toUnit(v), mapDBError(e)
}

func (s *UnitStore) List(ctx context.Context, l, o int32) ([]unit.Unit, int64, error) {
	vs, e := s.queries.ListUnits(ctx, codegen.ListUnitsParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountUnits(ctx)
	out := make([]unit.Unit, len(vs))
	for i, v := range vs {
		out[i] = toUnit(v)
	}
	return out, total, e
}

func (s *UnitStore) Update(ctx context.Context, id int64, c unit.UpdateRequest) (out unit.Unit, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		old, e := q.GetUnitByIDForUpdate(ctx, id)
		if e != nil {
			return e
		}
		v, e := q.UpdateUnit(ctx, codegen.UpdateUnitParams{ID: id, Description: c.Description, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toUnit(v)
		return audit(ctx, q, entityUnit, strconv.FormatInt(id, 10), policy.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}

func (s *UnitStore) Delete(ctx context.Context, id int64, actor string) error {
	return transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.DeleteUnit(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityUnit, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

var _ unit.Store = (*UnitStore)(nil)
