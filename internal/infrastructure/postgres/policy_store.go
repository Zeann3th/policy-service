package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	codegen "github.com/zeann3th/policy-service/internal/infrastructure/postgres/codegen"
)

// Store implements policy.Store on Postgres.
type Store struct {
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

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{
		db:              db,
		resolverQueries: codegen.New(db),
		queries:         codegen.New(db),
	}
}

func (s *Store) GetGroupID(
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

func (s *Store) GetActiveCustomPolicies(ctx context.Context, walletID string) ([]policy.CustomPolicy, error) {
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
			TransactionType: row.TransactionType,
			Channel:         row.Channel,
			Direction:       policy.Direction(row.Direction),
			WindowType:      policy.WindowType(row.WindowType),
			Unit:            fromText[policy.Unit](row.Unit),
			Base:            fromText[policy.Base](row.Base),
			Operator:        fromText[policy.Operator](row.Operator),
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

func (s *Store) GetPoliciesByGroupID(ctx context.Context, groupID int64) ([]policy.Policy, error) {
	rows, err := s.resolverQueries.GetPoliciesByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return toPolicies(rows), nil
}

func (s *Store) GetNextPolicyStart(ctx context.Context, groupID int64) (*time.Time, error) {
	value, err := s.resolverQueries.GetNextPolicyStart(ctx, groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &value, nil
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
			TransactionType: row.TransactionType,
			Channel:         row.Channel,
			Direction:       policy.Direction(row.Direction),
			WindowType:      policy.WindowType(row.WindowType),
			Unit:            fromText[policy.Unit](row.Unit),
			Base:            fromText[policy.Base](row.Base),
			Operator:        fromText[policy.Operator](row.Operator),
			Value:           row.Value,
			EffectiveFrom:   row.EffectiveFrom,
			EffectiveTo:     row.EffectiveTo,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
		}
	}
	return values
}

const (
	entityPolicyType   = "policy_type"
	entityPolicyGroup  = "policy_group"
	entityPolicy       = "policy"
	entityCustomPolicy = "custom_policy"
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
			return core.ErrConflict.WithCause(err)
		case "23514":
			return core.ErrBadRequest.WithCause(err)
		}
	}
	return err
}

func (s *Store) transact(ctx context.Context, fn func(*codegen.Queries) error) error {
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

func audit(ctx context.Context, q *codegen.Queries, entity, key string, action policy.AuditAction, oldValue, newValue any, actor string) error {
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
	_, err = q.CreatePolicyAuditLog(ctx, codegen.CreatePolicyAuditLogParams{Entity: entity, EntityKey: key, Action: string(action), OldValue: oldJSON, NewValue: newJSON, UpdatedBy: actor})
	return err
}

func toPolicyType(v codegen.PolicyType) policy.PolicyType {
	return policy.PolicyType{ID: v.ID, Code: v.Code, Description: v.Description, Kind: policy.Kind(v.Kind), Active: v.Active, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

// fromText reads a nullable text column; NULL is the zero value.
func fromText[T ~string](value *string) T {
	if value == nil {
		return ""
	}
	return T(*value)
}

// text writes a nullable text column; the zero value is NULL.
func text[T ~string](value T) *string {
	if value == "" {
		return nil
	}
	result := string(value)
	return &result
}

func toPolicyGroup(v codegen.PolicyGroup) policy.PolicyGroup {
	return policy.PolicyGroup{ID: v.ID, WalletType: v.WalletType, Currency: v.Currency, Tier: v.Tier, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func toPolicy(v codegen.Policy) policy.Policy {
	return policy.Policy{ID: v.ID, PolicyGroupID: v.PolicyGroupID, PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: policy.Direction(v.Direction), WindowType: policy.WindowType(v.WindowType), Unit: fromText[policy.Unit](v.Unit), Base: fromText[policy.Base](v.Base), Operator: fromText[policy.Operator](v.Operator), Value: v.Value, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func toCustomPolicy(v codegen.CustomPolicy) policy.CustomPolicy {
	return policy.CustomPolicy{ID: v.ID, WalletID: v.WalletID, PolicyTypeID: v.PolicyTypeID, TransactionType: v.TransactionType, Channel: v.Channel, Direction: policy.Direction(v.Direction), WindowType: policy.WindowType(v.WindowType), Unit: fromText[policy.Unit](v.Unit), Base: fromText[policy.Base](v.Base), Operator: fromText[policy.Operator](v.Operator), Value: v.Value, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}

func (s *Store) CreateType(ctx context.Context, c policy.CreatePolicyTypeRequest) (out policy.PolicyType, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreatePolicyType(ctx, codegen.CreatePolicyTypeParams{Code: c.Code, Description: c.Description, Kind: string(c.Kind), Active: *c.Active})
		if e != nil {
			return e
		}
		out = toPolicyType(v)
		return audit(ctx, q, entityPolicyType, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *Store) GetType(ctx context.Context, id int64) (policy.PolicyType, error) {
	v, e := s.queries.GetPolicyTypeByID(ctx, id)
	return toPolicyType(v), mapDBError(e)
}

func (s *Store) ListTypes(ctx context.Context, l, o int32) ([]policy.PolicyType, int64, error) {
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

func (s *Store) UpdateType(ctx context.Context, id int64, c policy.UpdatePolicyTypeRequest) (out policy.PolicyType, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
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

func (s *Store) DeleteType(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeletePolicyType(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityPolicyType, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

func (s *Store) CreateGroup(ctx context.Context, c policy.CreatePolicyGroupRequest) (out policy.PolicyGroup, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreatePolicyGroup(ctx, codegen.CreatePolicyGroupParams{WalletType: c.WalletType, Currency: c.Currency, Tier: c.Tier})
		if e != nil {
			return e
		}
		out = toPolicyGroup(v)
		return audit(ctx, q, entityPolicyGroup, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *Store) GetGroup(ctx context.Context, id int64) (policy.PolicyGroup, error) {
	v, e := s.queries.GetPolicyGroupByID(ctx, id)
	return toPolicyGroup(v), mapDBError(e)
}

func (s *Store) ListGroups(ctx context.Context, l, o int32) ([]policy.PolicyGroup, int64, error) {
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

func (s *Store) DeleteGroup(ctx context.Context, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeletePolicyGroup(ctx, id)
		if e != nil {
			return e
		}
		return audit(ctx, q, entityPolicyGroup, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

func (s *Store) CreatePolicy(ctx context.Context, g int64, c policy.CreatePolicyRequest) (out policy.Policy, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreatePolicy(ctx, codegen.CreatePolicyParams{PolicyGroupID: g, PolicyTypeID: c.PolicyTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Direction: string(c.Direction), WindowType: string(c.WindowType), Unit: text(c.Unit), Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo})
		if e != nil {
			return e
		}
		out = toPolicy(v)
		return audit(ctx, q, entityPolicy, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *Store) GetPolicy(ctx context.Context, g, id int64) (policy.Policy, error) {
	v, e := s.queries.GetPolicyByID(ctx, codegen.GetPolicyByIDParams{PolicyGroupID: g, ID: id})
	return toPolicy(v), mapDBError(e)
}

func (s *Store) ListPolicies(ctx context.Context, g int64, l, o int32) ([]policy.Policy, int64, error) {
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

func (s *Store) UpdatePolicy(ctx context.Context, g, id int64, c policy.UpdatePolicyRequest) (out policy.Policy, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		old, e := q.GetPolicyByIDForUpdate(ctx, codegen.GetPolicyByIDForUpdateParams{PolicyGroupID: g, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdatePolicy(ctx, codegen.UpdatePolicyParams{PolicyGroupID: g, ID: id, Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, EffectiveFrom: *c.EffectiveFrom, EffectiveTo: c.EffectiveTo})
		if e != nil {
			return e
		}
		out = toPolicy(v)
		return audit(ctx, q, entityPolicy, strconv.FormatInt(id, 10), policy.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}

func (s *Store) DeletePolicy(ctx context.Context, g, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeletePolicy(ctx, codegen.DeletePolicyParams{PolicyGroupID: g, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityPolicy, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

func (s *Store) CreateCustomPolicy(ctx context.Context, w string, c policy.CreateCustomPolicyRequest) (out policy.CustomPolicy, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.CreateCustomPolicy(ctx, codegen.CreateCustomPolicyParams{WalletID: w, PolicyTypeID: c.PolicyTypeID, TransactionType: c.TransactionType, Channel: c.Channel, Direction: string(c.Direction), WindowType: string(c.WindowType), Unit: text(c.Unit), Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, EffectiveFrom: c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason, CreatedBy: c.UpdatedBy})
		if e != nil {
			return e
		}
		out = toCustomPolicy(v)
		return audit(ctx, q, entityCustomPolicy, strconv.FormatInt(v.ID, 10), policy.AuditInsert, nil, v, c.UpdatedBy)
	})
	return
}

func (s *Store) GetCustomPolicy(ctx context.Context, w string, id int64) (policy.CustomPolicy, error) {
	v, e := s.queries.GetCustomPolicyByID(ctx, codegen.GetCustomPolicyByIDParams{WalletID: w, ID: id})
	return toCustomPolicy(v), mapDBError(e)
}

func (s *Store) ListCustomPolicies(ctx context.Context, w string, l, o int32) ([]policy.CustomPolicy, int64, error) {
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

func (s *Store) UpdateCustomPolicy(ctx context.Context, w string, id int64, c policy.UpdateCustomPolicyRequest) (out policy.CustomPolicy, err error) {
	err = s.transact(ctx, func(q *codegen.Queries) error {
		old, e := q.GetCustomPolicyByIDForUpdate(ctx, codegen.GetCustomPolicyByIDForUpdateParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		v, e := q.UpdateCustomPolicy(ctx, codegen.UpdateCustomPolicyParams{WalletID: w, ID: id, Base: text(c.Base), Operator: text(c.Operator), Value: *c.Value, EffectiveFrom: *c.EffectiveFrom, EffectiveTo: c.EffectiveTo, Reason: c.Reason})
		if e != nil {
			return e
		}
		out = toCustomPolicy(v)
		return audit(ctx, q, entityCustomPolicy, strconv.FormatInt(id, 10), policy.AuditUpdate, old, v, c.UpdatedBy)
	})
	return
}

func (s *Store) DeleteCustomPolicy(ctx context.Context, w string, id int64, actor string) error {
	return s.transact(ctx, func(q *codegen.Queries) error {
		v, e := q.DeleteCustomPolicy(ctx, codegen.DeleteCustomPolicyParams{WalletID: w, ID: id})
		if e != nil {
			return e
		}
		return audit(ctx, q, entityCustomPolicy, strconv.FormatInt(id, 10), policy.AuditDelete, v, nil, actor)
	})
}

var _ policy.Store = (*Store)(nil)
