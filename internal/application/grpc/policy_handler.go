package grpc

import (
	"context"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyHandler struct {
	pb.UnimplementedPolicyServiceServer
	service policy.Service
}

func (h *policyHandler) GetPolicyConfig(ctx context.Context, request *pb.GetPolicyConfigRequest) (*pb.GetPolicyConfigResponse, error) {
	if err := require(map[string]string{"walletType": request.WalletType, "currency": request.Currency, "tier": request.Tier, "walletId": request.WalletId}); err != nil {
		return nil, err
	}
	if err := isoCurrency(request.Currency); err != nil {
		return nil, err
	}
	response, err := h.service.GetConfig(ctx, policy.GetConfigRequest{WalletType: request.WalletType, Currency: request.Currency, Tier: request.Tier, WalletID: request.WalletId, TransactionType: request.TransactionType})
	if err != nil {
		return nil, grpcError(err)
	}
	result := &pb.GetPolicyConfigResponse{Rules: make([]*pb.PolicyRule, len(response.Rules))}
	for i, value := range response.Rules {
		result.Rules[i] = &pb.PolicyRule{PolicyType: value.PolicyType, Kind: string(value.Kind), Unit: string(value.Unit), Base: string(value.Base), TransactionType: value.TransactionType, Channel: value.Channel, Direction: string(value.Direction), WindowType: string(value.WindowType), Operator: string(value.Operator), Value: value.Value.String(), Source: string(value.Source)}
	}
	return result, nil
}

func (h *policyHandler) CreatePolicy(ctx context.Context, request *pb.CreatePolicyRequest) (*pb.Policy, error) {
	if err := mutation(request.UpdatedBy, request.GroupId, request.PolicyTypeId); err != nil {
		return nil, err
	}
	if err := operator(request.Operator); err != nil {
		return nil, err
	}
	if err := direction(request.Direction); err != nil {
		return nil, err
	}
	if err := unitCode(request.Unit); err != nil {
		return nil, err
	}
	if err := base(request.Base); err != nil {
		return nil, err
	}
	if err := windowType(request.WindowType); err != nil {
		return nil, err
	}
	groupID, _ := intID(request.GroupId)
	typeID, _ := intID(request.PolicyTypeId)
	value, err := dec(request.Value)
	if err != nil {
		return nil, err
	}
	from, err := ts(request.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	to, err := ts(request.EffectiveTo)
	if err != nil {
		return nil, err
	}
	response, err := h.service.CreatePolicy(ctx, groupID, policy.CreatePolicyRequest{PolicyTypeID: typeID, TransactionType: request.TransactionType, Channel: request.Channel, Direction: policy.Direction(request.Direction), WindowType: policy.WindowType(request.WindowType), Unit: policy.Unit(request.Unit), Base: policy.Base(request.Base), Operator: policy.Operator(request.Operator), Value: &value, EffectiveFrom: from, EffectiveTo: to, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicy(response), nil
}

func (h *policyHandler) GetPolicy(ctx context.Context, request *pb.ScopedIdRequest) (*pb.Policy, error) {
	groupID, id, err := scopedIDs(request.ParentId, request.Id)
	if err != nil {
		return nil, err
	}
	response, err := h.service.GetPolicy(ctx, groupID, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicy(response), nil
}

func (h *policyHandler) ListPolicies(ctx context.Context, request *pb.ScopedPageRequest) (*pb.PolicyPage, error) {
	groupID, err := intID(request.ParentId)
	if err != nil {
		return nil, err
	}
	pageRequest, err := page(request.Page, request.Limit)
	if err != nil {
		return nil, err
	}
	response, err := h.service.ListPolicies(ctx, groupID, pageRequest)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &pb.PolicyPage{Items: make([]*pb.Policy, len(response.Items)), Page: response.Page, Limit: response.Limit, Total: response.Total}
	for i, value := range response.Items {
		result.Items[i] = outPolicy(value)
	}
	return result, nil
}

func (h *policyHandler) UpdatePolicy(ctx context.Context, request *pb.UpdatePolicyRequest) (*pb.Policy, error) {
	if err := mutation(request.UpdatedBy, request.GroupId, request.Id); err != nil {
		return nil, err
	}
	if err := operator(request.Operator); err != nil {
		return nil, err
	}
	if err := base(request.Base); err != nil {
		return nil, err
	}
	groupID, _ := intID(request.GroupId)
	id, _ := intID(request.Id)
	value, err := dec(request.Value)
	if err != nil {
		return nil, err
	}
	from, err := ts(request.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	to, err := ts(request.EffectiveTo)
	if err != nil {
		return nil, err
	}
	response, err := h.service.UpdatePolicy(ctx, groupID, id, policy.UpdatePolicyRequest{Base: policy.Base(request.Base), Operator: policy.Operator(request.Operator), Value: &value, EffectiveFrom: from, EffectiveTo: to, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicy(response), nil
}

func (h *policyHandler) DeletePolicy(ctx context.Context, request *pb.DeleteScopedRequest) (*pb.DeleteResponse, error) {
	if err := mutation(request.UpdatedBy, request.ParentId, request.Id); err != nil {
		return nil, err
	}
	groupID, _ := intID(request.ParentId)
	id, _ := intID(request.Id)
	if err := h.service.DeletePolicy(ctx, groupID, id, core.ActorRequest{UpdatedBy: request.UpdatedBy}); err != nil {
		return nil, grpcError(err)
	}
	return &pb.DeleteResponse{}, nil
}

func (h *policyHandler) CreateCustomPolicy(ctx context.Context, request *pb.CreateCustomPolicyRequest) (*pb.CustomPolicy, error) {
	if err := require(map[string]string{"walletId": request.WalletId}); err != nil {
		return nil, err
	}
	if err := mutation(request.UpdatedBy, request.PolicyTypeId); err != nil {
		return nil, err
	}
	if err := operator(request.Operator); err != nil {
		return nil, err
	}
	if err := direction(request.Direction); err != nil {
		return nil, err
	}
	if err := unitCode(request.Unit); err != nil {
		return nil, err
	}
	if err := base(request.Base); err != nil {
		return nil, err
	}
	if err := windowType(request.WindowType); err != nil {
		return nil, err
	}
	typeID, _ := intID(request.PolicyTypeId)
	value, err := dec(request.Value)
	if err != nil {
		return nil, err
	}
	from, err := ts(request.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	to, err := ts(request.EffectiveTo)
	if err != nil {
		return nil, err
	}
	response, err := h.service.CreateCustomPolicy(ctx, request.WalletId, policy.CreateCustomPolicyRequest{PolicyTypeID: typeID, TransactionType: request.TransactionType, Channel: request.Channel, Direction: policy.Direction(request.Direction), WindowType: policy.WindowType(request.WindowType), Unit: policy.Unit(request.Unit), Base: policy.Base(request.Base), Operator: policy.Operator(request.Operator), Value: &value, EffectiveFrom: from, EffectiveTo: to, Reason: request.Reason, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outCustomPolicy(response), nil
}

func (h *policyHandler) GetCustomPolicy(ctx context.Context, request *pb.WalletIdRequest) (*pb.CustomPolicy, error) {
	if err := require(map[string]string{"walletId": request.WalletId}); err != nil {
		return nil, err
	}
	id, err := intID(request.Id)
	if err != nil {
		return nil, err
	}
	response, err := h.service.GetCustomPolicy(ctx, request.WalletId, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return outCustomPolicy(response), nil
}

func (h *policyHandler) ListCustomPolicies(ctx context.Context, request *pb.WalletPageRequest) (*pb.CustomPolicyPage, error) {
	if err := require(map[string]string{"walletId": request.WalletId}); err != nil {
		return nil, err
	}
	pageRequest, err := page(request.Page, request.Limit)
	if err != nil {
		return nil, err
	}
	response, err := h.service.ListCustomPolicies(ctx, request.WalletId, pageRequest)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &pb.CustomPolicyPage{Items: make([]*pb.CustomPolicy, len(response.Items)), Page: response.Page, Limit: response.Limit, Total: response.Total}
	for i, value := range response.Items {
		result.Items[i] = outCustomPolicy(value)
	}
	return result, nil
}

func (h *policyHandler) UpdateCustomPolicy(ctx context.Context, request *pb.UpdateCustomPolicyRequest) (*pb.CustomPolicy, error) {
	if err := require(map[string]string{"walletId": request.WalletId}); err != nil {
		return nil, err
	}
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	if err := operator(request.Operator); err != nil {
		return nil, err
	}
	if err := base(request.Base); err != nil {
		return nil, err
	}
	id, _ := intID(request.Id)
	value, err := dec(request.Value)
	if err != nil {
		return nil, err
	}
	from, err := ts(request.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	if from == nil {
		return nil, grpcError(core.ErrBadRequest.WithMessage("effectiveFrom is required"))
	}
	to, err := ts(request.EffectiveTo)
	if err != nil {
		return nil, err
	}
	response, err := h.service.UpdateCustomPolicy(ctx, request.WalletId, id, policy.UpdateCustomPolicyRequest{Base: policy.Base(request.Base), Operator: policy.Operator(request.Operator), Value: &value, EffectiveFrom: from, EffectiveTo: to, Reason: request.Reason, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outCustomPolicy(response), nil
}

func (h *policyHandler) DeleteCustomPolicy(ctx context.Context, request *pb.DeleteCustomPolicyRequest) (*pb.DeleteResponse, error) {
	if err := require(map[string]string{"walletId": request.WalletId}); err != nil {
		return nil, err
	}
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	id, _ := intID(request.Id)
	if err := h.service.DeleteCustomPolicy(ctx, request.WalletId, id, core.ActorRequest{UpdatedBy: request.UpdatedBy}); err != nil {
		return nil, grpcError(err)
	}
	return &pb.DeleteResponse{}, nil
}

func scopedIDs(parent, id uint64) (int64, int64, error) {
	parentID, err := intID(parent)
	if err != nil {
		return 0, 0, err
	}
	itemID, err := intID(id)
	return parentID, itemID, err
}
