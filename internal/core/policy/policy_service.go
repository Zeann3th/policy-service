package policy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	"github.com/zeann3th/policy-service/internal/core/unit"
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
	store Store
	units unit.Store
	cache *cachecore.Versioned
}

func NewService(store Store, units unit.Store, cache *cachecore.Versioned) Service {
	return &service{store: store, units: units, cache: cache}
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
		Operator:        value.Operator,
		Value:           value.Value,
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
		Operator:        value.Operator,
		Value:           value.Value,
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
	if len(policies) == 0 {
		return response, nil
	}
	units, err := s.getActiveUnits(ctx)
	if err != nil {
		return GetConfigResponse{}, err
	}
	for i, value := range policies {
		unitCode, ok := units[value.UnitID]
		if !ok {
			return GetConfigResponse{}, fmt.Errorf("unit %d not found", value.UnitID)
		}
		response.Rules[i] = Rule{
			PolicyType:      value.PolicyType,
			Unit:            unitCode,
			TransactionType: value.TransactionType,
			Channel:         value.Channel,
			Direction:       value.Direction,
			WindowType:      value.WindowType,
			Operator:        value.Operator,
			Value:           value.Value,
			Source:          value.Source,
		}
	}
	return response, nil
}

// Resolve returns the effective policies (system merged with the wallet's
// custom overrides) that cover one side of a transaction, with unit codes.
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
	units, err := s.getActiveUnits(ctx)
	if err != nil {
		return nil, err
	}
	matched := make([]Policy, 0)
	for _, value := range mergePolicies(systemPolicies, customPolicies) {
		if !value.Matches(request.TransactionType, request.Channel, request.Direction) {
			continue
		}
		unitCode, ok := units[value.UnitID]
		if !ok {
			return nil, fmt.Errorf("unit %d not found", value.UnitID)
		}
		value.Unit = unitCode
		matched = append(matched, value)
	}
	return matched, nil
}

func (s *service) getActiveUnits(ctx context.Context) (map[int64]string, error) {
	key := s.cache.Key(ctx, cachecore.NamespaceUnits, "active")
	if cached, ok := cachecore.Read[map[int64]string](ctx, s.cache, key); ok {
		return cached, nil
	}
	items, err := s.units.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active units: %w", err)
	}
	units := make(map[int64]string, len(items))
	for _, item := range items {
		units[item.ID] = item.Code
	}
	cachecore.Write(ctx, s.cache, key, units)
	return units, nil
}

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
	cachecore.Write(ctx, s.cache, key, policies)
	return policies, nil
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
			UnitID:          customPolicy.UnitID,
			TransactionType: customPolicy.TransactionType,
			Channel:         customPolicy.Channel,
			Direction:       customPolicy.Direction,
			WindowType:      customPolicy.WindowType,
			Operator:        customPolicy.Operator,
			Value:           customPolicy.Value,
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
	if request.Value == nil {
		return PolicyResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	request.TransactionType, request.Channel = defaults(request.TransactionType, request.Channel)
	request.Direction = defaultDirection(request.Direction)
	if request.WindowType == "" {
		request.WindowType = WindowNone
	}
	value, err := s.store.CreatePolicy(ctx, groupID, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return policyResponse(value), err
}

func (s *service) GetPolicy(ctx context.Context, groupID, id int64) (PolicyResponse, error) {
	key := s.cache.Key(ctx, cachecore.NamespacePolicies, "item", groupID, id)
	if cached, ok := cachecore.Read[PolicyResponse](ctx, s.cache, key); ok {
		return cached, nil
	}
	value, err := s.store.GetPolicy(ctx, groupID, id)
	response := policyResponse(value)
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *service) ListPolicies(ctx context.Context, groupID int64, request core.PageRequest) (core.Page[PolicyResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespacePolicies, "list", groupID, page, limit)
	if cached, ok := cachecore.Read[core.Page[PolicyResponse]](ctx, s.cache, key); ok {
		return cached, nil
	}
	values, total, err := s.store.ListPolicies(ctx, groupID, limit, offset)
	response := core.Page[PolicyResponse]{Items: make([]PolicyResponse, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		response.Items[i] = policyResponse(value)
	}
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *service) UpdatePolicy(ctx context.Context, groupID, id int64, request UpdatePolicyRequest) (PolicyResponse, error) {
	if request.Value == nil {
		return PolicyResponse{}, core.ErrBadRequest.WithMessage("value is required")
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

func validWindow(from time.Time, to *time.Time) bool {
	return to == nil || from.Before(*to)
}

func (s *service) CreateCustomPolicy(ctx context.Context, walletID string, request CreateCustomPolicyRequest) (CustomPolicyResponse, error) {
	if request.Value == nil {
		return CustomPolicyResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	if request.EffectiveFrom != nil && !validWindow(*request.EffectiveFrom, request.EffectiveTo) {
		return CustomPolicyResponse{}, core.ErrBadRequest.WithMessage("effectiveFrom must be before effectiveTo")
	}
	if request.EffectiveFrom == nil && request.EffectiveTo != nil && !time.Now().Before(*request.EffectiveTo) {
		return CustomPolicyResponse{}, core.ErrBadRequest.WithMessage("effectiveTo must be in the future when effectiveFrom is omitted")
	}
	request.TransactionType, request.Channel = defaults(request.TransactionType, request.Channel)
	request.Direction = defaultDirection(request.Direction)
	if request.WindowType == "" {
		request.WindowType = WindowNone
	}
	value, err := s.store.CreateCustomPolicy(ctx, walletID, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return customPolicyResponse(value), err
}

func (s *service) GetCustomPolicy(ctx context.Context, walletID string, id int64) (CustomPolicyResponse, error) {
	key := s.cache.Key(ctx, cachecore.NamespaceCustomPolicies, "item", walletID, id)
	if cached, ok := cachecore.Read[CustomPolicyResponse](ctx, s.cache, key); ok {
		return cached, nil
	}
	value, err := s.store.GetCustomPolicy(ctx, walletID, id)
	response := customPolicyResponse(value)
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *service) ListCustomPolicies(ctx context.Context, walletID string, request core.PageRequest) (core.Page[CustomPolicyResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespaceCustomPolicies, "list", walletID, page, limit)
	if cached, ok := cachecore.Read[core.Page[CustomPolicyResponse]](ctx, s.cache, key); ok {
		return cached, nil
	}
	values, total, err := s.store.ListCustomPolicies(ctx, walletID, limit, offset)
	response := core.Page[CustomPolicyResponse]{Items: make([]CustomPolicyResponse, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		response.Items[i] = customPolicyResponse(value)
	}
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *service) UpdateCustomPolicy(ctx context.Context, walletID string, id int64, request UpdateCustomPolicyRequest) (CustomPolicyResponse, error) {
	if request.Value == nil {
		return CustomPolicyResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	if request.EffectiveFrom == nil {
		return CustomPolicyResponse{}, core.ErrBadRequest.WithMessage("effectiveFrom is required")
	}
	if !validWindow(*request.EffectiveFrom, request.EffectiveTo) {
		return CustomPolicyResponse{}, core.ErrBadRequest.WithMessage("effectiveFrom must be before effectiveTo")
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
