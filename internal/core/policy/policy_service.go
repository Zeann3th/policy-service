package policy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
	cacheport "github.com/zeann3th/policy-service/internal/core/port/cache"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

const cacheTTL = 15 * time.Minute

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
}

type service struct {
	logger      *slog.Logger
	store       Store
	unitStore   unit.Store
	cache       cacheport.Cache
	serviceName string
}

func NewServices(
	logger *slog.Logger,
	store Store,
	unitStore unit.Store,
	cache cacheport.Cache,
	serviceName string,
) (GroupService, TypeService, Service) {
	service := &service{logger: logger, store: store, unitStore: unitStore, cache: cache, serviceName: serviceName}
	return &groupService{service}, &typeService{service}, service
}

func (s *service) generation(ctx context.Context, namespace string) string {
	generation, err := s.cache.Get(ctx, cachecore.GenerationKey(s.serviceName, namespace))
	if err == nil {
		return generation
	}
	if errors.Is(err, cacheport.ErrMiss) {
		return "0"
	}
	s.logger.Warn("get policy cache generation", "namespace", namespace, "error", err)
	return fmt.Sprintf("bypass-%d", time.Now().UnixNano())
}

func (s *service) key(ctx context.Context, namespace string, parts ...any) string {
	args := []any{s.serviceName, namespace, s.generation(ctx, namespace)}
	args = append(args, parts...)
	return core.Join(args...)
}

func cacheRead[T any](s *service, ctx context.Context, key string) (T, bool) {
	var zero T
	encoded, err := s.cache.Get(ctx, key)
	if err != nil {
		return zero, false
	}
	value, err := core.Decode[T](encoded)
	if err != nil {
		s.logger.Warn("decode policy cache", "key", key, "error", err)
		return zero, false
	}
	return *value, true
}

func cacheWrite[T any](s *service, ctx context.Context, key string, value T) {
	encoded, err := core.Encode(&value)
	if err == nil {
		err = s.cache.SetWithTTL(ctx, key, encoded, cacheTTL)
	}
	if err != nil {
		s.logger.Warn("cache policy response", "key", key, "error", err)
	}
}

func (s *service) invalidate(ctx context.Context, namespaces ...string) {
	for _, namespace := range namespaces {
		if _, err := s.cache.Increment(ctx, cachecore.GenerationKey(s.serviceName, namespace)); err != nil {
			s.logger.Warn("invalidate policy cache", "namespace", namespace, "error", err)
		}
	}
}

func policyResponse(value Policy) PolicyResponse {
	return PolicyResponse{
		ID:              value.ID,
		PolicyGroupID:   value.PolicyGroupID,
		PolicyTypeID:    value.PolicyTypeID,
		TransactionType: value.TransactionType,
		Channel:         value.Channel,
		Direction:       value.Direction,
		Operator:        value.Operator,
		Value:           value.Value,
		CreatedAt:       value.CreatedAt,
		UpdatedAt:       value.UpdatedAt,
	}
}

func customPolicyResponse(value CustomPolicy) CustomPolicyResponse {
	return CustomPolicyResponse{
		ID:              value.ID,
		WalletID:        value.WalletID,
		PolicyTypeID:    value.PolicyTypeID,
		TransactionType: value.TransactionType,
		Channel:         value.Channel,
		Direction:       value.Direction,
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
			Operator:        value.Operator,
			Value:           value.Value,
			Source:          value.Source,
		}
	}
	return response, nil
}

func (s *service) getActiveUnits(ctx context.Context) (map[int64]string, error) {
	key := s.key(ctx, cachecore.NamespaceUnits, "active")
	if cached, ok := cacheRead[map[int64]string](s, ctx, key); ok {
		return cached, nil
	}
	items, err := s.unitStore.GetActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active units: %w", err)
	}
	units := make(map[int64]string, len(items))
	for _, item := range items {
		units[item.ID] = item.Code
	}
	cacheWrite(s, ctx, key, units)
	return units, nil
}

func (s *service) getSystemPolicies(ctx context.Context, request GetConfigRequest) ([]Policy, error) {
	key := s.key(ctx, cachecore.NamespacePolicyConfig, request.WalletType, request.Currency, request.Tier)
	if cached, ok := cacheRead[[]Policy](s, ctx, key); ok {
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
	cacheWrite(s, ctx, key, policies)
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
			PolicyTypeID:    customPolicy.PolicyTypeID,
			PolicyType:      customPolicy.PolicyType,
			UnitID:          customPolicy.UnitID,
			TransactionType: customPolicy.TransactionType,
			Channel:         customPolicy.Channel,
			Direction:       customPolicy.Direction,
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
	value, err := s.store.CreatePolicy(ctx, groupID, request)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return policyResponse(value), err
}

func (s *service) GetPolicy(ctx context.Context, groupID, id int64) (PolicyResponse, error) {
	key := s.key(ctx, cachecore.NamespacePolicies, "item", groupID, id)
	if cached, ok := cacheRead[PolicyResponse](s, ctx, key); ok {
		return cached, nil
	}
	value, err := s.store.GetPolicy(ctx, groupID, id)
	response := policyResponse(value)
	if err == nil {
		cacheWrite(s, ctx, key, response)
	}
	return response, err
}

func (s *service) ListPolicies(ctx context.Context, groupID int64, request core.PageRequest) (core.Page[PolicyResponse], error) {
	page, limit, offset := request.Values()
	key := s.key(ctx, cachecore.NamespacePolicies, "list", groupID, page, limit)
	if cached, ok := cacheRead[core.Page[PolicyResponse]](s, ctx, key); ok {
		return cached, nil
	}
	values, total, err := s.store.ListPolicies(ctx, groupID, limit, offset)
	response := core.Page[PolicyResponse]{Items: make([]PolicyResponse, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		response.Items[i] = policyResponse(value)
	}
	if err == nil {
		cacheWrite(s, ctx, key, response)
	}
	return response, err
}

func (s *service) UpdatePolicy(ctx context.Context, groupID, id int64, request UpdatePolicyRequest) (PolicyResponse, error) {
	if request.Value == nil {
		return PolicyResponse{}, core.ErrBadRequest.WithMessage("value is required")
	}
	value, err := s.store.UpdatePolicy(ctx, groupID, id, request)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return policyResponse(value), err
}

func (s *service) DeletePolicy(ctx context.Context, groupID, id int64, request core.ActorRequest) error {
	err := s.store.DeletePolicy(ctx, groupID, id, request.UpdatedBy)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
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
	value, err := s.store.CreateCustomPolicy(ctx, walletID, request)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return customPolicyResponse(value), err
}

func (s *service) GetCustomPolicy(ctx context.Context, walletID string, id int64) (CustomPolicyResponse, error) {
	key := s.key(ctx, cachecore.NamespaceCustomPolicies, "item", walletID, id)
	if cached, ok := cacheRead[CustomPolicyResponse](s, ctx, key); ok {
		return cached, nil
	}
	value, err := s.store.GetCustomPolicy(ctx, walletID, id)
	response := customPolicyResponse(value)
	if err == nil {
		cacheWrite(s, ctx, key, response)
	}
	return response, err
}

func (s *service) ListCustomPolicies(ctx context.Context, walletID string, request core.PageRequest) (core.Page[CustomPolicyResponse], error) {
	page, limit, offset := request.Values()
	key := s.key(ctx, cachecore.NamespaceCustomPolicies, "list", walletID, page, limit)
	if cached, ok := cacheRead[core.Page[CustomPolicyResponse]](s, ctx, key); ok {
		return cached, nil
	}
	values, total, err := s.store.ListCustomPolicies(ctx, walletID, limit, offset)
	response := core.Page[CustomPolicyResponse]{Items: make([]CustomPolicyResponse, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		response.Items[i] = customPolicyResponse(value)
	}
	if err == nil {
		cacheWrite(s, ctx, key, response)
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
		s.invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return customPolicyResponse(value), err
}

func (s *service) DeleteCustomPolicy(ctx context.Context, walletID string, id int64, request core.ActorRequest) error {
	err := s.store.DeleteCustomPolicy(ctx, walletID, id, request.UpdatedBy)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return err
}
