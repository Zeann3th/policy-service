package policy

import (
	"context"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type GroupService interface {
	Create(context.Context, CreatePolicyGroupRequest) (PolicyGroupResponse, error)
	Get(context.Context, int64) (PolicyGroupResponse, error)
	List(context.Context, core.PageRequest) (core.Page[PolicyGroupResponse], error)
	Delete(context.Context, int64, core.ActorRequest) error
}

type groupService struct {
	store Store
	cache *cachecore.Versioned
}

func NewGroupService(store Store, cache *cachecore.Versioned) GroupService {
	return &groupService{store: store, cache: cache}
}

func policyGroupResponse(value PolicyGroup) PolicyGroupResponse {
	return PolicyGroupResponse{
		ID:         value.ID,
		WalletType: value.WalletType,
		Currency:   value.Currency,
		Tier:       value.Tier,
		CreatedAt:  value.CreatedAt,
		UpdatedAt:  value.UpdatedAt,
	}
}

func (s *groupService) Create(ctx context.Context, request CreatePolicyGroupRequest) (PolicyGroupResponse, error) {
	value, err := s.store.CreateGroup(ctx, request)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicyGroups)
	}
	return policyGroupResponse(value), err
}

func (s *groupService) Get(ctx context.Context, id int64) (PolicyGroupResponse, error) {
	key := s.cache.Key(ctx, cachecore.NamespacePolicyGroups, "item", id)
	if cached, ok := cachecore.Read[PolicyGroupResponse](ctx, s.cache, key); ok {
		return cached, nil
	}
	value, err := s.store.GetGroup(ctx, id)
	response := policyGroupResponse(value)
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *groupService) List(ctx context.Context, request core.PageRequest) (core.Page[PolicyGroupResponse], error) {
	page, limit, offset := request.Values()
	key := s.cache.Key(ctx, cachecore.NamespacePolicyGroups, "list", page, limit)
	if cached, ok := cachecore.Read[core.Page[PolicyGroupResponse]](ctx, s.cache, key); ok {
		return cached, nil
	}
	values, total, err := s.store.ListGroups(ctx, limit, offset)
	response := core.Page[PolicyGroupResponse]{Items: make([]PolicyGroupResponse, len(values)), Page: page, Limit: limit, Total: total}
	for i, value := range values {
		response.Items[i] = policyGroupResponse(value)
	}
	if err == nil {
		cachecore.Write(ctx, s.cache, key, response)
	}
	return response, err
}

func (s *groupService) Delete(ctx context.Context, id int64, request core.ActorRequest) error {
	err := s.store.DeleteGroup(ctx, id, request.UpdatedBy)
	if err == nil {
		s.cache.Invalidate(ctx, cachecore.NamespacePolicyGroups, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return err
}
