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
	store TypeStore
	cache *cachecore.Versioned
}

func NewTypeService(store TypeStore, cache *cachecore.Versioned) TypeService {
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
	code, ok := core.Code(request.Code)
	if !ok {
		return PolicyTypeResponse{}, core.ErrBadRequest.WithMessage("code must be letters, digits and underscores, starting with a letter")
	}
	request.Code = code
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
	return cachecore.Fetch(ctx, s.cache, key, func() (PolicyTypeResponse, error) {
		value, err := s.store.GetType(ctx, id)
		return policyTypeResponse(value), err
	})
}

func (s *typeService) List(ctx context.Context, request core.PageRequest) (core.Page[PolicyTypeResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespacePolicyTypes, "list", page, limit)
	return cachecore.Fetch(ctx, s.cache, key, func() (core.Page[PolicyTypeResponse], error) {
		values, total, err := s.store.ListTypes(ctx, limit, offset)
		return core.MapPage(values, page, limit, total, policyTypeResponse), err
	})
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
