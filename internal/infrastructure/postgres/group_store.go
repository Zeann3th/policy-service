package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core/policy"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// GroupStore implements policy.GroupStore: wallet type x currency x tier.
type GroupStore struct {
	db      *pgxpool.Pool
	queries *codegen.Queries
}

func NewGroupStore(db *pgxpool.Pool) *GroupStore {
	return &GroupStore{db: db, queries: codegen.New(db)}
}

func toPolicyGroup(v codegen.PolicyGroup) policy.PolicyGroup {
	return policy.PolicyGroup{ID: v.ID, WalletType: v.WalletType, Currency: v.Currency, Tier: v.Tier, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *GroupStore) CreateGroup(ctx context.Context, c policy.CreatePolicyGroupRequest) (out policy.PolicyGroup, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.CreatePolicyGroup(ctx, codegen.CreatePolicyGroupParams{WalletType: c.WalletType, Currency: c.Currency, Tier: c.Tier})
		if e != nil {
			return e
		}
		out = toPolicyGroup(v)
		return audit(ctx, q, entityPolicyGroup, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *GroupStore) GetGroup(ctx context.Context, id int64) (policy.PolicyGroup, error) {
	v, e := s.queries.GetPolicyGroupByID(ctx, id)
	return toPolicyGroup(v), mapDBError(e)
}

func (s *GroupStore) ListGroups(ctx context.Context, l, o int32) ([]policy.PolicyGroup, int64, error) {
	vs, e := s.queries.ListPolicyGroups(ctx, codegen.ListPolicyGroupsParams{PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountPolicyGroups(ctx)
	out := make([]policy.PolicyGroup, len(vs))
	for i, v := range vs {
		out[i] = toPolicyGroup(v)
	}
	return out, total, e
}

func (s *GroupStore) DeleteGroup(ctx context.Context, id int64, actor string) error {
	return transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.DeletePolicyGroup(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityPolicyGroup, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

var _ policy.GroupStore = (*GroupStore)(nil)
