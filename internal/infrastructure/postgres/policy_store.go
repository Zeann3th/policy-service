package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// PolicyStore implements policy.PolicyStore: resolving a wallet's
// policies, the catalog reads policies are validated against, and system
// and custom policies.
type PolicyStore struct {
	db              *pgxpool.Pool
	resolverQueries policyQueries
	queries         *codegen.Queries
}

type policyQueries interface {
	GetPolicyGroupID(context.Context, codegen.GetPolicyGroupIDParams) (int64, error)
	GetPoliciesByGroupID(context.Context, int64) ([]codegen.GetPoliciesByGroupIDRow, error)
	GetNextPolicyStart(context.Context, int64) (time.Time, error)
	GetActiveCustomPolicies(context.Context, string) ([]codegen.GetActiveCustomPoliciesRow, error)
}

func NewPolicyStore(db *pgxpool.Pool) *PolicyStore {
	return &PolicyStore{db: db, resolverQueries: codegen.New(db), queries: codegen.New(db)}
}

func (s *PolicyStore) GetGroupID(
	ctx context.Context,
	walletType string,
	currency string,
	tier string,
) (int64, error) {
	groupID, err := s.resolverQueries.GetPolicyGroupID(ctx, codegen.GetPolicyGroupIDParams{
		WalletType: walletType,
		Currency:   currency,
		Tier:       tier,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, core.ErrNotFound.WithCause(err)
	}
	return groupID, err
}

func (s *PolicyStore) GetActiveCustomPolicies(ctx context.Context, walletID string) ([]policy.CustomPolicy, error) {
	rows, err := s.resolverQueries.GetActiveCustomPolicies(ctx, walletID)
	if err != nil {
		return nil, err
	}
	values := make([]policy.CustomPolicy, len(rows))
	for i, row := range rows {
		values[i] = policy.CustomPolicy{
			ID:              row.ID,
			WalletID:        row.WalletID,
			PolicyTypeID:    row.PolicyTypeID,
			PolicyType:      row.PolicyType,
			Kind:            policy.Kind(row.Kind),
			Role:            fromText[policy.Role](row.Role),
			TransactionType: row.TransactionType,
			Channel:         row.Channel,
			Direction:       policy.Direction(row.Direction),
			WindowType:      policy.WindowType(row.WindowType),
			UnitID:          row.UnitID,
			Unit:            fromText[string](row.Unit),
			Base:            fromText[policy.Base](row.Base),
			Operator:        fromText[policy.Operator](row.Operator),
			Cap:             row.Cap,
			IsEnforced:      row.IsEnforced,
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

func (s *PolicyStore) GetPoliciesByGroupID(ctx context.Context, groupID int64) ([]policy.Policy, error) {
	rows, err := s.resolverQueries.GetPoliciesByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return toPolicies(rows), nil
}

func (s *PolicyStore) GetNextPolicyStart(ctx context.Context, groupID int64) (*time.Time, error) {
	value, err := s.resolverQueries.GetNextPolicyStart(ctx, groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *PolicyStore) GetUnitCode(ctx context.Context, id int64) (string, error) {
	v, e := s.queries.GetUnitByID(ctx, id)
	return v.Code, mapDBError(e)
}

func (s *PolicyStore) GetType(ctx context.Context, id int64) (policy.PolicyType, error) {
	v, e := s.queries.GetPolicyTypeByID(ctx, id)
	return toPolicyType(v), mapDBError(e)
}

func toPolicies(rows []codegen.GetPoliciesByGroupIDRow) []policy.Policy {
	values := make([]policy.Policy, len(rows))
	for i, row := range rows {
		values[i] = policy.Policy{
			ID:              row.ID,
			PolicyGroupID:   row.PolicyGroupID,
			PolicyTypeID:    row.PolicyTypeID,
			PolicyType:      row.PolicyType,
			Kind:            policy.Kind(row.Kind),
			Role:            fromText[policy.Role](row.Role),
			TransactionType: row.TransactionType,
			Channel:         row.Channel,
			Direction:       policy.Direction(row.Direction),
			WindowType:      policy.WindowType(row.WindowType),
			UnitID:          row.UnitID,
			Unit:            fromText[string](row.Unit),
			Base:            fromText[policy.Base](row.Base),
			Operator:        fromText[policy.Operator](row.Operator),
			Cap:             row.Cap,
			IsEnforced:      row.IsEnforced,
			Value:           row.Value,
			EffectiveFrom:   row.EffectiveFrom,
			EffectiveTo:     row.EffectiveTo,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
	}
	return values
}

func toPolicy(v codegen.Policy) policy.Policy {
	return policy.Policy{ID: v.ID, PolicyGroupID: v.PolicyGroupID, PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: policy.Direction(v.Direction), WindowType: policy.WindowType(v.WindowType), Kind: policy.Kind(v.Kind), Role: fromText[policy.Role](v.Role), IsEnforced: v.IsEnforced, UnitID: v.UnitID, Base: fromText[policy.Base](v.Base), Operator: fromText[policy.Operator](v.Operator), Value: v.Value, Cap: v.Cap, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func toCustomPolicy(v codegen.CustomPolicy) policy.CustomPolicy {
	return policy.CustomPolicy{ID: v.ID, WalletID: v.WalletID, PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: policy.Direction(v.Direction), WindowType: policy.WindowType(v.WindowType), Kind: policy.Kind(v.Kind), Role: fromText[policy.Role](v.Role), IsEnforced: v.IsEnforced, UnitID: v.UnitID, Base: fromText[policy.Base](v.Base), Operator: fromText[policy.Operator](v.Operator), Value: v.Value, Cap: v.Cap, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *PolicyStore) CreatePolicy(ctx context.Context, g int64, c policy.CreatePolicyRequest) (out policy.Policy, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.CreatePolicy(ctx, codegen.CreatePolicyParams{PolicyGroupID: g, PolicyTypeID: c.PolicyTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Direction: string(c.Direction), WindowType: string(c.WindowType), Kind: string(c.Kind), Role: text(c.Role), IsEnforced: *c.IsEnforced, UnitID: c.UnitID, Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, Cap: c.Cap, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo})
		if e != nil {
			return e
		}
		out = toPolicy(v)
		return audit(ctx, q, entityPolicy, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *PolicyStore) GetPolicy(ctx context.Context, g, id int64) (policy.Policy, error) {
	v, e := s.queries.GetPolicyByID(ctx, codegen.GetPolicyByIDParams{PolicyGroupID: g, ID: id})
	return toPolicy(v), mapDBError(e)
}

func (s *PolicyStore) ListPolicies(ctx context.Context, g int64, l, o int32) ([]policy.Policy, int64, error) {
	vs, e := s.queries.ListPolicies(ctx, codegen.ListPoliciesParams{PolicyGroupID: g, PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountPolicies(ctx, g)
	out := make([]policy.Policy, len(vs))
	for i, v := range vs {
		out[i] = toPolicy(v)
	}
	return out, total, e
}

func (s *PolicyStore) UpdatePolicy(ctx context.Context, g, id int64, c policy.UpdatePolicyRequest) (out policy.Policy, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		old, e := q.GetPolicyByIDForUpdate(ctx, codegen.GetPolicyByIDForUpdateParams{PolicyGroupID: g, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdatePolicy(ctx, codegen.UpdatePolicyParams{PolicyGroupID: g, ID: id, IsEnforced: *c.IsEnforced, Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, Cap: c.Cap, EffectiveFrom: *c.EffectiveFrom, EffectiveTo: c.EffectiveTo})
		if e != nil {
			return e
		}
		out = toPolicy(v)
		return audit(ctx, q, entityPolicy, strconv.FormatInt(id, 10), policy.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}

func (s *PolicyStore) DeletePolicy(ctx context.Context, g, id int64, actor string) error {
	return transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.DeletePolicy(ctx, codegen.DeletePolicyParams{PolicyGroupID: g, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityPolicy, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

func (s *PolicyStore) CreateCustomPolicy(ctx context.Context, w string, c policy.CreateCustomPolicyRequest) (out policy.CustomPolicy, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.CreateCustomPolicy(ctx, codegen.CreateCustomPolicyParams{WalletID: w, PolicyTypeID: c.PolicyTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Direction: string(c.Direction), WindowType: string(c.WindowType), Kind: string(c.Kind), Role: text(c.Role), IsEnforced: *c.IsEnforced, UnitID: c.UnitID, Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, Cap: c.Cap, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason, CreatedBy: c.UpdatedBy})
		if e != nil {
			return e
		}
		out = toCustomPolicy(v)
		return audit(ctx, q, entityCustomPolicy, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *PolicyStore) GetCustomPolicy(ctx context.Context, w string, id int64) (policy.CustomPolicy, error) {
	v, e := s.queries.GetCustomPolicyByID(ctx, codegen.GetCustomPolicyByIDParams{WalletID: w, ID: id})
	return toCustomPolicy(v), mapDBError(e)
}

func (s *PolicyStore) ListCustomPolicies(ctx context.Context, w string, l, o int32) ([]policy.CustomPolicy, int64, error) {
	vs, e := s.queries.ListCustomPolicies(ctx, codegen.ListCustomPoliciesParams{WalletID: w, PageLimit: l, PageOffset: o})
	if e != nil {
		return nil, 0, e
	}
	total, e := s.queries.CountCustomPolicies(ctx, w)
	out := make([]policy.CustomPolicy, len(vs))
	for i, v := range vs {
		out[i] = toCustomPolicy(v)
	}
	return out, total, e
}

func (s *PolicyStore) UpdateCustomPolicy(ctx context.Context, w string, id int64, c policy.UpdateCustomPolicyRequest) (out policy.CustomPolicy, err error) {
	err = transact(ctx, s.db, func(q *codegen.Queries) error {
		old, e := q.GetCustomPolicyByIDForUpdate(ctx, codegen.GetCustomPolicyByIDForUpdateParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdateCustomPolicy(ctx, codegen.UpdateCustomPolicyParams{WalletID: w, ID: id, IsEnforced: *c.IsEnforced, Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, Cap: c.Cap, EffectiveFrom: *c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason})
		if e != nil {
			return e
		}
		out = toCustomPolicy(v)
		return audit(ctx, q, entityCustomPolicy, strconv.FormatInt(id, 10), policy.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}

func (s *PolicyStore) DeleteCustomPolicy(ctx context.Context, w string, id int64, actor string) error {
	return transact(ctx, s.db, func(q *codegen.Queries) error {
		v, e := q.DeleteCustomPolicy(ctx, codegen.DeleteCustomPolicyParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityCustomPolicy, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

var _ policy.PolicyStore = (*PolicyStore)(nil)
