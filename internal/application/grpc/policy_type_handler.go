package grpc

import (
	"context"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type policyTypeHandler struct {
	pb.UnimplementedPolicyTypeServiceServer
	service policy.TypeService
}

func (h *policyTypeHandler) CreatePolicyType(ctx context.Context, request *pb.CreatePolicyTypeRequest) (*pb.PolicyType, error) {
	if err := require(map[string]string{"code": request.Code, "updatedBy": request.UpdatedBy}); err != nil {
		return nil, err
	}
	if err := kind(request.Kind); err != nil {
		return nil, err
	}
	response, err := h.service.Create(ctx, policy.CreatePolicyTypeRequest{Code: request.Code, Description: request.Description, Kind: policy.Kind(request.Kind), Active: request.Active, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicyType(response), nil
}

func (h *policyTypeHandler) GetPolicyType(ctx context.Context, request *pb.IdRequest) (*pb.PolicyType, error) {
	id, err := intID(request.Id)
	if err != nil {
		return nil, err
	}
	response, err := h.service.Get(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicyType(response), nil
}

func (h *policyTypeHandler) ListPolicyTypes(ctx context.Context, request *pb.PageRequest) (*pb.PolicyTypePage, error) {
	pageRequest, err := page(request.Page, request.Limit)
	if err != nil {
		return nil, err
	}
	response, err := h.service.List(ctx, pageRequest)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &pb.PolicyTypePage{Items: make([]*pb.PolicyType, len(response.Items)), Page: response.Page, Limit: response.Limit, Total: response.Total}
	for i, value := range response.Items {
		result.Items[i] = outPolicyType(value)
	}
	return result, nil
}

func (h *policyTypeHandler) UpdatePolicyType(ctx context.Context, request *pb.UpdatePolicyTypeRequest) (*pb.PolicyType, error) {
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	if request.Active == nil {
		return nil, grpcError(core.ErrBadRequest.WithMessage("active is required"))
	}
	id, _ := intID(request.Id)
	response, err := h.service.Update(ctx, id, policy.UpdatePolicyTypeRequest{Description: request.Description, Active: request.Active, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outPolicyType(response), nil
}

func (h *policyTypeHandler) DeletePolicyType(ctx context.Context, request *pb.DeleteByIdRequest) (*pb.DeleteResponse, error) {
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	id, _ := intID(request.Id)
	if err := h.service.Delete(ctx, id, core.ActorRequest{UpdatedBy: request.UpdatedBy}); err != nil {
		return nil, grpcError(err)
	}
	return &pb.DeleteResponse{}, nil
}
