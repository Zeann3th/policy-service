package policy

import (
	"context"

	"github.com/zeann3th/policy-service/internal/core"
	cachecore "github.com/zeann3th/policy-service/internal/core/cache"
)

type GroupService interface {
	Create(context.Context, CreatePolicyGroupRequest) (PolicyGroupResponse, error)
	Get(context.Context, int64) (PolicyGroupResponse, error)
	Delete(context.Context, int64, core.ActorRequest) error
}

type groupService struct {
	*service
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
		s.invalidate(ctx, cachecore.NamespacePolicyGroups)
	}
	return policyGroupResponse(value), err
}

func (s *groupService) Get(ctx context.Context, id int64) (PolicyGroupResponse, error) {
	key := s.key(ctx, cachecore.NamespacePolicyGroups, "item", id)
	if cached, ok := cacheRead[PolicyGroupResponse](s.service, ctx, key); ok {
		return cached, nil
	}
	value, err := s.store.GetGroup(ctx, id)
	response := policyGroupResponse(value)
	if err == nil {
		cacheWrite(s.service, ctx, key, response)
	}
	return response, err
}

func (s *groupService) Delete(ctx context.Context, id int64, request core.ActorRequest) error {
	err := s.store.DeleteGroup(ctx, id, request.UpdatedBy)
	if err == nil {
		s.invalidate(ctx, cachecore.NamespacePolicyGroups, cachecore.NamespacePolicies, cachecore.NamespacePolicyConfig)
	}
	return err
}
