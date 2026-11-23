package policy

import (
	"context"
	"fmt"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type TypeService interface {
	Create(context.Context, CreatePolicyTypeRequest) (PolicyTypeResponse, error)
	Get(context.Context, int64) (PolicyTypeResponse, error)
	List(context.Context, core.PageRequest) (core.Page[PolicyTypeResponse], error)
	Update(context.Context, int64, UpdatePolicyTypeRequest) (PolicyTypeResponse, error)
	Delete(context.Context, int64, core.ActorRequest) error
}

type typeService struct {
	store Store
	cache *cachecore.Versioned
}

func NewTypeService(store Store, cache *cachecore.Versioned) TypeService {
	return &typeService{store: store, cache: cache}
}

func policyTypeResponse(value PolicyType) PolicyTypeResponse {
	return PolicyTypeResponse{
		ID:          value.ID,
		Code:        value.Code,
		Description: value.Description,
		Kind:        value.Kind,
		Active:      value.Active,
		CreatedAt:   value.CreatedAt,
		UpdatedAt:   value.UpdatedAt,
	}
}

func (s *typeService) Create(ctx context.Context, request CreatePolicyTypeRequest) (PolicyTypeResponse, error) {
	if request.Active == nil {
		active := true
		request.Active = &active
	}
	if request.Kind == "" {
		request.Kind = KindLimit
	}
	if !request.Kind.Valid() {
		return PolicyTypeResponse{}, core.ErrBadRequest.WithMessage(fmt.Sprintf("kind must be one of %v", Kinds()))
	}
	value, err := s.store.CreateType(ctx, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicyTypes)
	}
	return policyTypeResponse(value), err
}

func (s *typeService) Get(ctx context.Context, id int64) (PolicyTypeResponse, error) {
	key := s.cache.Key(ctx, cachecore.NamespacePolicyTypes, "item", id)
	if cached, ok := cachecore.Read[PolicyTypeResponse](ctx, s.cache, key); ok {
		return cached, nil
	}
	value, err := s.store.GetType(ctx, id)
	response := policyTypeResponse(value)
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *typeService) List(ctx context.Context, request core.PageRequest) (core.Page[PolicyTypeResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespacePolicyTypes, "list", page, limit)
	if cached, ok := cachecore.Read[core.Page[PolicyTypeResponse]](ctx, s.cache, key); ok {
		return cached, nil
	}
	values, total, err := s.store.ListTypes(ctx, limit, offset)
	response := core.Page[PolicyTypeResponse]{Items: make([]PolicyTypeResponse, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		response.Items[i] = policyTypeResponse(value)
	}
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *typeService) Update(ctx context.Context, id int64, request UpdatePolicyTypeRequest) (PolicyTypeResponse, error) {
	if request.Active == nil {
		return PolicyTypeResponse{}, core.ErrBadRequest.WithMessage("active is required")
	}
	value, err := s.store.UpdateType(ctx, id, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicyTypes, cachecore.NamespacePolicyConfig)
	}
	return policyTypeResponse(value), err
}

func (s *typeService) Delete(ctx context.Context, id int64, request core.ActorRequest) error {
	err := s.store.DeleteType(ctx, id, request.UpdatedBy)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicyTypes, cachecore.NamespacePolicies, cachecore.NamespaceCustomPolicies, cachecore.NamespacePolicyConfig)
	}
	return err
}
