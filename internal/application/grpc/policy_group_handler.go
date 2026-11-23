package grpc

import (
	"context"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyGroupHandler struct {
	pb.UnimplementedPolicyGroupServiceServer
	service policy.GroupService
}

func (h *policyGroupHandler) CreatePolicyGroup(ctx context.Context, request *pb.CreatePolicyGroupRequest) (*pb.PolicyGroup, error) {
	if err := require(map[string]string{"walletType": request.WalletType, "currency": request.Currency, "tier": request.Tier, "updatedBy": request.UpdatedBy}); err != nil {
		return nil, err
	}
	if err := isoCurrency(request.Currency); err != nil {
		return nil, err
	}
	response, err := h.service.Create(ctx, policy.CreatePolicyGroupRequest{WalletType: request.WalletType, Currency: request.Currency, Tier: request.Tier, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicyGroup(response), nil
}

func (h *policyGroupHandler) ListPolicyGroups(ctx context.Context, request *pb.PageRequest) (*pb.PolicyGroupPage, error) {
	pageRequest, err := page(request.Page, request.Limit)
	if err != nil {
		return nil, err
	}
	response, err := h.service.List(ctx, pageRequest)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &pb.PolicyGroupPage{Items: make([]*pb.PolicyGroup, len(response.Items)), Page: response.Page, Limit: response.Limit, Total: response.Total}
	for i, value := range response.Items {
		result.Items[i] = outPolicyGroup(value)
	}
	return result, nil
}

func (h *policyGroupHandler) GetPolicyGroup(ctx context.Context, request *pb.IdRequest) (*pb.PolicyGroup, error) {
	id, err := intID(request.Id)
	if err != nil {
		return nil, err
	}
	response, err := h.service.Get(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicyGroup(response), nil
}

func (h *policyGroupHandler) DeletePolicyGroup(ctx context.Context, request *pb.DeleteByIdRequest) (*pb.DeleteResponse, error) {
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	id, _ := intID(request.Id)
	if err := h.service.Delete(ctx, id, core.ActorRequest{UpdatedBy: request.UpdatedBy}); err != nil {
		return nil, grpcError(err)
	}
	return &pb.DeleteResponse{}, nil
}
