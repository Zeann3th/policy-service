package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core/policy"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// TypeStore implements policy.TypeStore: the policy types catalog.
type TypeStore struct {
	db      *pgxpool.Pool
	queries *codegen.Queries
}

func NewTypeStore(db *pgxpool.Pool) *TypeStore {
	return &TypeStore{db: db, queries: codegen.New(db)}
}

func toPolicyType(v codegen.PolicyType) policy.PolicyType {
	return policy.PolicyType{ID: v.ID, Code: v.Code, Description: v.Description, Kind: policy.Kind(v.Kind), Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *TypeStore) CreateType(ctx context.Context, c policy.CreatePolicyTypeRequest) (out policy.PolicyType, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.CreatePolicyType(ctx, codegen.CreatePolicyTypeParams{Code: c.Code, Description: c.Description, Kind: string(c.Kind), Active: *c.Active})
		if e != nil {
			return e
		}
		out = toPolicyType(v)
		return audit(ctx, q, entityPolicyType, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *TypeStore) GetType(ctx context.Context, id int64) (policy.PolicyType, error) {
	v, e := s.queries.GetPolicyTypeByID(ctx, id)
	return toPolicyType(v), mapDBError(e)
}

func (s *TypeStore) ListTypes(ctx context.Context, l, o int32) ([]policy.PolicyType, int64, error) {
	vs, e := s.queries.ListPolicyTypes(ctx, codegen.ListPolicyTypesParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountPolicyTypes(ctx)
	out := make([]policy.PolicyType, len(vs))
	for i, v := range vs {
		out[i] = toPolicyType(v)
	}
	return out, total, e
}

func (s *TypeStore) UpdateType(ctx context.Context, id int64, c policy.UpdatePolicyTypeRequest) (out policy.PolicyType, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		old, e := q.GetPolicyTypeByIDForUpdate(ctx, id)
		if e != nil {
			return e
		}
		v, e := q.UpdatePolicyType(ctx, codegen.UpdatePolicyTypeParams{ID: id, Description: c.Description, Active: *c.Active})
		if e != nil {
			return e
		}
		out = toPolicyType(v)
		return audit(ctx, q, entityPolicyType, strconv.FormatInt(id, 10), policy.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}

func (s *TypeStore) DeleteType(ctx context.Context, id int64, actor string) error {
	return transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.DeletePolicyType(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityPolicyType, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

var _ policy.TypeStore = (*TypeStore)(nil)
