package policy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type Service interface {
	GetConfig(context.Context, GetConfigRequest) (GetConfigResponse, error)
	CreatePolicy(context.Context, int64, CreatePolicyRequest) (PolicyResponse, error)
	GetPolicy(context.Context, int64, int64) (PolicyResponse, error)
	ListPolicies(context.Context, int64, core.PageRequest) (core.Page[PolicyResponse], error)
	UpdatePolicy(context.Context, int64, int64, UpdatePolicyRequest) (PolicyResponse, error)
	DeletePolicy(context.Context, int64, int64, core.ActorRequest) error
	CreateCustomPolicy(context.Context, string, CreateCustomPolicyRequest) (CustomPolicyResponse, error)
	GetCustomPolicy(context.Context, string, int64) (CustomPolicyResponse, error)
	ListCustomPolicies(context.Context, string, core.PageRequest) (core.Page[CustomPolicyResponse], error)
	UpdateCustomPolicy(context.Context, string, int64, UpdateCustomPolicyRequest) (CustomPolicyResponse, error)
	DeleteCustomPolicy(context.Context, string, int64, core.ActorRequest) error
	Resolve(context.Context, ResolveRequest) ([]Policy, error)
}

type service struct {
	store PolicyStore
	rules Rules
	cache *cachecore.Versioned
}

func NewService(store PolicyStore, rules Rules, cache *cachecore.Versioned) Service {
	return &service{store: store, rules: rules, cache: cache}
}

func policyResponse(value Policy) PolicyResponse {
	if value.WindowType == "" {
		value.WindowType = WindowNone
	}
	return PolicyResponse{
		ID:              value.ID,
		PolicyGroupID:   value.PolicyGroupID,
		PolicyTypeID:    value.PolicyTypeID,
		TransactionType: value.TransactionType,
		Channel:         value.Channel,
		Direction:       value.Direction,
		WindowType:      value.WindowType,
		Kind:            value.Kind,
		Role:            value.Role,
		IsEnforced:      value.IsEnforced,
		UnitID:          value.UnitID,
		Base:            value.Base,
		Operator:        value.Operator,
		Value:           value.Value,
		Cap:             value.Cap,
		EffectiveFrom:   value.EffectiveFrom,
		EffectiveTo:     value.EffectiveTo,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}

func customPolicyResponse(value CustomPolicy) CustomPolicyResponse {
	if value.WindowType == "" {
		value.WindowType = WindowNone
	}
	return CustomPolicyResponse{
		ID:              value.ID,
		WalletID:        value.WalletID,
		PolicyTypeID:    value.PolicyTypeID,
		TransactionType: value.TransactionType,
		Channel:         value.Channel,
		Direction:       value.Direction,
		WindowType:      value.WindowType,
		Kind:            value.Kind,
		Role:            value.Role,
		IsEnforced:      value.IsEnforced,
		UnitID:          value.UnitID,
		Base:            value.Base,
		Operator:        value.Operator,
		Value:           value.Value,
		Cap:             value.Cap,
		EffectiveFrom:   value.EffectiveFrom,
		EffectiveTo:     value.EffectiveTo,
		Reason:          value.Reason,
		CreatedBy:       value.CreatedBy,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}

func (s *service) GetConfig(ctx context.Context, request GetConfigRequest) (GetConfigResponse, error) {
	policies, err := s.getSystemPolicies(ctx, request)
	if err != nil {
		return GetConfigResponse{}, err
	}
	customPolicies, err := s.store.GetActiveCustomPolicies(ctx, request.WalletID)
	if err != nil {
		return GetConfigResponse{}, fmt.Errorf("get active custom policies: %w", err)
	}
	policies = mergePolicies(policies, customPolicies)
	policies = filterPoliciesByTransactionType(policies, request.TransactionType)

	response := GetConfigResponse{Rules: make([]Rule, len(policies))}
	for i, value := range policies {
		response.Rules[i] = Rule{
			PolicyType:      value.PolicyType,
			Kind:            value.Kind,
			Role:            value.Role,
			TransactionType: value.TransactionType,
			Channel:         value.Channel,
			Direction:       value.Direction,
			WindowType:      value.WindowType,
			Unit:            value.Unit,
			Base:            value.Base,
			Operator:        value.Operator,
			Value:           value.Value,
			Cap:             value.Cap,
			Source:          value.Source,
		}
	}
	return response, nil
}

// Resolve returns the effective policies (system merged with the wallet's
// custom overrides) that cover one side of a transaction.
// A wallet without a policy group gets only its custom policies.
func (s *service) Resolve(ctx context.Context, request ResolveRequest) ([]Policy, error) {
	systemPolicies, err := s.getSystemPolicies(ctx, GetConfigRequest{WalletType: request.WalletType, Currency: request.Currency, Tier: request.Tier})
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}
	customPolicies, err := s.store.GetActiveCustomPolicies(ctx, request.WalletID)
	if err != nil {
		return nil, fmt.Errorf("get active custom policies: %w", err)
	}
	matched := make([]Policy, 0)
	for _, value := range mergePolicies(systemPolicies, customPolicies) {
		if value.Matches(request.TransactionType, request.Channel, request.Direction) {
			matched = append(matched, value)
		}
	}
	return matched, nil
}

// getSystemPolicies returns the group's policies effective now. The cached
// list expires at its next effective boundary, so a policy starting or ending
// while it is cached still takes effect on time.
func (s *service) getSystemPolicies(ctx context.Context, request GetConfigRequest) ([]Policy, error) {
	key := s.cache.Key(ctx, cachecore.NamespacePolicyConfig, request.WalletType, request.Currency, request.Tier)
	if cached, ok := cachecore.Read[[]Policy](ctx, s.cache, key); ok {
		return cached, nil
	}
	groupID, err := s.store.GetGroupID(ctx, request.WalletType, request.Currency, request.Tier)
	if err != nil {
		return nil, fmt.Errorf("get policy group: %w", err)
	}
	policies, err := s.store.GetPoliciesByGroupID(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get policies: %w", err)
	}
	nextStart, err := s.store.GetNextPolicyStart(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("get next policy start: %w", err)
	}
	if ttl := cacheTTL(policies, nextStart, time.Now()); ttl > 0 {
		cachecore.WriteTTL(ctx, s.cache, key, policies, ttl)
	}
	return policies, nil
}

// cacheTTL bounds the cache TTL by the next effective boundary: the next
// policy start or the earliest end among the effective policies.
func cacheTTL(policies []Policy, nextStart *time.Time, now time.Time) time.Duration {
	boundary := now.Add(cachecore.TTL)
	if nextStart != nil && nextStart.Before(boundary) {
		boundary = *nextStart
	}
	for _, value := range policies {
		if value.EffectiveTo != nil && value.EffectiveTo.Before(boundary) {
			boundary = *value.EffectiveTo
		}
	}
	return boundary.Sub(now)
}

func filterPoliciesByTransactionType(values []Policy, transactionType string) []Policy {
	if transactionType == "" {
		return values
	}
	filtered := make([]Policy, 0, len(values))
	for _, value := range values {
		if value.TransactionType == All || value.TransactionType == transactionType {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func mergePolicies(systemPolicies []Policy, customPolicies []CustomPolicy) []Policy {
	values := append([]Policy(nil), systemPolicies...)
	indexes := make(map[PolicyKey]int, len(values))
	for i, value := range values {
		values[i].Source = SourceSystem
		indexes[value.Key()] = i
	}
	for _, customPolicy := range customPolicies {
		value := Policy{
			ID:              customPolicy.ID,
			PolicyTypeID:    customPolicy.PolicyTypeID,
			PolicyType:      customPolicy.PolicyType,
			Kind:            customPolicy.Kind,
			Role:            customPolicy.Role,
			TransactionType: customPolicy.TransactionType,
			Channel:         customPolicy.Channel,
			Direction:       customPolicy.Direction,
			WindowType:      customPolicy.WindowType,
			UnitID:          customPolicy.UnitID,
			Unit:            customPolicy.Unit,
			Base:            customPolicy.Base,
			Operator:        customPolicy.Operator,
			Value:           customPolicy.Value,
			Cap:             customPolicy.Cap,
			IsEnforced:      customPolicy.IsEnforced,
			Source:          SourceCustom,
		}
		if index, ok := indexes[customPolicy.Key()]; ok {
			values[index] = value
			continue
		}
		indexes[customPolicy.Key()] = len(values)
		values = append(values, value)
	}
	return values
}

func defaults(transactionType, channel string) (string, string) {
	if transactionType == "" {
		transactionType = All
	}
	if channel == "" {
		channel = All
	}
	return transactionType, channel
}

func defaultDirection(direction Direction) Direction {
	if direction == "" {
		return DirectionAll
	}
	return direction
}

func (s *service) CreatePolicy(ctx context.Context, groupID int64, request CreatePolicyRequest) (PolicyResponse, error) {
	if err := s.prepareNew(ctx, &request.PolicyFields); err != nil {
		return PolicyResponse{}, err
	}
	value, err := s.store.CreatePolicy(ctx, groupID, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return policyResponse(value), err
}

func (s *service) GetPolicy(ctx context.Context, groupID, id int64) (PolicyResponse, error) {
	key := s.cache.Key(ctx, cachecore.NamespacePolicies, "item", groupID, id)
	return cachecore.Fetch(ctx, s.cache, key, func() (PolicyResponse, error) {
		value, err := s.store.GetPolicy(ctx, groupID, id)
		return policyResponse(value), err
	})
}

func (s *service) ListPolicies(ctx context.Context, groupID int64, request core.PageRequest) (core.Page[PolicyResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespacePolicies, "list", groupID, page, limit)
	return cachecore.Fetch(ctx, s.cache, key, func() (core.Page[PolicyResponse], error) {
		values, total, err := s.store.ListPolicies(ctx, groupID, limit, offset)
		return core.MapPage(values, page, limit, total, policyResponse), err
	})
}

func (s *service) UpdatePolicy(ctx context.Context, groupID, id int64, request UpdatePolicyRequest) (PolicyResponse, error) {
	current, err := s.store.GetPolicy(ctx, groupID, id)
	if err != nil {
		return PolicyResponse{}, err
	}
	if err := s.checkChange(ctx, current.identity(), &request.PolicyChange); err != nil {
		return PolicyResponse{}, err
	}
	value, err := s.store.UpdatePolicy(ctx, groupID, id, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return policyResponse(value), err
}

func (s *service) DeletePolicy(ctx context.Context, groupID, id int64, request core.ActorRequest) error {
	err := s.store.DeletePolicy(ctx, groupID, id, request.UpdatedBy)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return err
}

// prepareNew fills the defaults of a new system or custom policy and
// validates it: value, effective range, and the rules of its type's kind.
func (s *service) prepareNew(ctx context.Context, fields *PolicyFields) error {
	if fields.Value == nil {
		return core.ErrBadRequest.WithMessage("value is required")
	}
	if err := validateEffective(fields.EffectiveFrom, fields.EffectiveTo); err != nil {
		return err
	}
	fields.TransactionType, fields.Channel = defaults(fields.TransactionType, fields.Channel)
	fields.Direction = defaultDirection(fields.Direction)
	if fields.WindowType == "" {
		fields.WindowType = WindowNone
	}
	if fields.IsEnforced == nil {
		enforced := true
		fields.IsEnforced = &enforced
	}
	return s.check(ctx, fields.PolicyTypeID, fields.UnitID, fields.spec(fields.Kind, fields.Role, fields.WindowType))
}

// identity is what an existing policy keeps when it changes, plus its
// current enforcement (kept unless the change sets it).
type identity struct {
	typeID     int64
	unitID     *int64
	window     WindowType
	kind       Kind
	role       Role
	isEnforced bool
}

func (v Policy) identity() identity {
	return identity{typeID: v.PolicyTypeID, unitID: v.UnitID, window: v.WindowType, kind: v.Kind, role: v.Role, isEnforced: v.IsEnforced}
}

func (v CustomPolicy) identity() identity {
	return identity{typeID: v.PolicyTypeID, unitID: v.UnitID, window: v.WindowType, kind: v.Kind, role: v.Role, isEnforced: v.IsEnforced}
}

// checkChange validates a change to an existing policy, whose identity
// stays as it is.
func (s *service) checkChange(ctx context.Context, current identity, change *PolicyChange) error {
	if change.Value == nil {
		return core.ErrBadRequest.WithMessage("value is required")
	}
	if change.EffectiveFrom == nil {
		return core.ErrBadRequest.WithMessage("effectiveFrom is required")
	}
	if !validWindow(*change.EffectiveFrom, change.EffectiveTo) {
		return core.ErrBadRequest.WithMessage("effectiveFrom must be before effectiveTo")
	}
	if change.IsEnforced == nil {
		change.IsEnforced = &current.isEnforced
	}
	return s.check(ctx, current.typeID, current.unitID, change.spec(current.kind, current.role, current.window))
}

// check validates a policy against the rules of its kind, with the type
// checked to exist and the unit code resolved from the catalog. A policy's
// identity is immutable, so reading the catalog before the write cannot
// race with it.
func (s *service) check(ctx context.Context, typeID int64, unitID *int64, spec Spec) error {
	_, err := s.store.GetType(ctx, typeID)
	if errors.Is(err, core.ErrNotFound) {
		return core.ErrBadRequest.WithMessage(fmt.Sprintf("policy type %d not found", typeID))
	}
	if err != nil {
		return err
	}
	if unitID != nil {
		code, err := s.store.GetUnitCode(ctx, *unitID)
		if errors.Is(err, core.ErrNotFound) {
			return core.ErrBadRequest.WithMessage(fmt.Sprintf("unit %d not found", *unitID))
		}
		if err != nil {
			return err
		}
		spec.Unit = code
	}
	if err := s.rules.Check(spec); err != nil {
		return core.ErrBadRequest.WithMessage(err.Error())
	}
	return nil
}

func validWindow(from time.Time, to *time.Time) bool {
	return to == nil || from.Before(*to)
}

// validateEffective checks an optional effective range on create; an omitted
// effectiveFrom defaults to now.
func validateEffective(from, to *time.Time) error {
	if from != nil && !validWindow(*from, to) {
		return core.ErrBadRequest.WithMessage("effectiveFrom must be before effectiveTo")
	}
	if from == nil && to != nil && !time.Now().Before(*to) {
		return core.ErrBadRequest.WithMessage("effectiveTo must be in the future when effectiveFrom is omitted")
	}
	return nil
}

func (s *service) CreateCustomPolicy(ctx context.Context, walletID string, request CreateCustomPolicyRequest) (CustomPolicyResponse, error) {
	if err := s.prepareNew(ctx, &request.PolicyFields); err != nil {
		return CustomPolicyResponse{}, err
	}
	value, err := s.store.CreateCustomPolicy(ctx, walletID, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return customPolicyResponse(value), err
}

func (s *service) GetCustomPolicy(ctx context.Context, walletID string, id int64) (CustomPolicyResponse, error) {
	key := s.cache.Key(ctx, cachecore.NamespaceCustomPolicies, "item", walletID, id)
	return cachecore.Fetch(ctx, s.cache, key, func() (CustomPolicyResponse, error) {
		value, err := s.store.GetCustomPolicy(ctx, walletID, id)
		return customPolicyResponse(value), err
	})
}

func (s *service) ListCustomPolicies(ctx context.Context, walletID string, request core.PageRequest) (core.Page[CustomPolicyResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespaceCustomPolicies, "list", walletID, page, limit)
	return cachecore.Fetch(ctx, s.cache, key, func() (core.Page[CustomPolicyResponse], error) {
		values, total, err := s.store.ListCustomPolicies(ctx, walletID, limit, offset)
		return core.MapPage(values, page, limit, total, customPolicyResponse), err
	})
}

func (s *service) UpdateCustomPolicy(ctx context.Context, walletID string, id int64, request UpdateCustomPolicyRequest) (CustomPolicyResponse, error) {
	current, err := s.store.GetCustomPolicy(ctx, walletID, id)
	if err != nil {
		return CustomPolicyResponse{}, err
	}
	if err := s.checkChange(ctx, current.identity(), &request.PolicyChange); err != nil {
		return CustomPolicyResponse{}, err
	}
	value, err := s.store.UpdateCustomPolicy(ctx, walletID, id, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return customPolicyResponse(value), err
}

func (s *service) DeleteCustomPolicy(ctx context.Context, walletID string, id int64, request core.ActorRequest) error {
	err := s.store.DeleteCustomPolicy(ctx, walletID, id, request.UpdatedBy)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return err
}

// spec is how the change reads its value, in a policy of kind and role
// with window.
func (c PolicyChange) spec(kind Kind, role Role, window WindowType) Spec {
	return Spec{Kind: kind, Role: role, Base: c.Base, Operator: c.Operator, Window: window, Value: *c.Value, Cap: c.Cap}
}
