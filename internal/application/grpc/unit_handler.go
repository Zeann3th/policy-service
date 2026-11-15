package grpc

import (
	"context"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

type unitHandler struct {
	pb.UnimplementedUnitServiceServer
	service unit.Service
}

func (h *unitHandler) CreateUnit(ctx context.Context, request *pb.CreateUnitRequest) (*pb.Unit, error) {
	if err := require(map[string]string{"code": request.Code, "updatedBy": request.UpdatedBy}); err != nil {
		return nil, err
	}
	response, err := h.service.Create(ctx, unit.CreateRequest{Code: request.Code, Active: request.Active, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outUnit(response), nil
}

func (h *unitHandler) GetUnit(ctx context.Context, request *pb.IdRequest) (*pb.Unit, error) {
	id, err := intID(request.Id)
	if err != nil {
		return nil, err
	}
	response, err := h.service.Get(ctx, id)
	if err != nil {
		return nil, grpcError(err)
	}
	return outUnit(response), nil
}

func (h *unitHandler) ListUnits(ctx context.Context, request *pb.PageRequest) (*pb.UnitPage, error) {
	pageRequest, err := page(request.Page, request.Limit)
	if err != nil {
		return nil, err
	}
	response, err := h.service.List(ctx, pageRequest)
	if err != nil {
		return nil, grpcError(err)
	}
	result := &pb.UnitPage{Items: make([]*pb.Unit, len(response.Items)), Page: response.Page, Limit: response.Limit, Total: response.Total}
	for i, value := range response.Items {
		result.Items[i] = outUnit(value)
	}
	return result, nil
}

func (h *unitHandler) UpdateUnit(ctx context.Context, request *pb.UpdateUnitRequest) (*pb.Unit, error) {
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	if request.Active == nil {
		return nil, grpcError(core.ErrBadRequest.WithMessage("active is required"))
	}
	id, err := intID(request.Id)
	if err != nil {
		return nil, err
	}
	response, err := h.service.Update(ctx, id, unit.UpdateRequest{Active: request.Active, UpdatedBy: request.UpdatedBy})
	if err != nil {
		return nil, grpcError(err)
	}
	return outUnit(response), nil
}

func (h *unitHandler) DeleteUnit(ctx context.Context, request *pb.DeleteByIdRequest) (*pb.DeleteResponse, error) {
	if err := mutation(request.UpdatedBy, request.Id); err != nil {
		return nil, err
	}
	id, err := intID(request.Id)
	if err != nil {
		return nil, err
	}
	if err := h.service.Delete(ctx, id, core.ActorRequest{UpdatedBy: request.UpdatedBy}); err != nil {
		return nil, grpcError(err)
	}
	return &pb.DeleteResponse{}, nil
}
