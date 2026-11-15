package grpc

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/limit"
	"golang.org/x/text/currency"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type limitGRPCHandler struct {
	pb.UnimplementedLimitServiceServer
	service limit.LimitService
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, core.ErrBadRequest):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, core.ErrNotFound):
		return status.Error(codes.NotFound, core.ErrNotFound.Message)
	case errors.Is(err, core.ErrConflict):
		return status.Error(codes.AlreadyExists, core.ErrConflict.Message)
	default:
		return status.Error(codes.Internal, core.ErrInternal.Message)
	}
}
func require(fields map[string]string) error {
	for n, v := range fields {
		if strings.TrimSpace(v) == "" {
			return status.Errorf(codes.InvalidArgument, "%s is required", n)
		}
	}
	return nil
}
func page(p, l int32) (limit.PageRequest, error) {
	if p < 0 || p > limit.MaxPage || l < 0 || l > 100 {
		return limit.PageRequest{}, status.Error(codes.InvalidArgument, "page must be positive and limit must be between 1 and 100")
	}
	return limit.PageRequest{Page: p, Limit: l}, nil
}
func dec(v string) (decimal.Decimal, error) {
	d, e := decimal.NewFromString(v)
	if e != nil {
		return d, status.Error(codes.InvalidArgument, "value must be a decimal")
	}
	return d, nil
}
func mutation(actor string, ids ...uint64) error {
	if strings.TrimSpace(actor) == "" {
		return status.Error(codes.InvalidArgument, "updatedBy is required")
	}
	for _, id := range ids {
		if id < 1 {
			return status.Error(codes.InvalidArgument, "id must be positive")
		}
	}
	return nil
}
func positive(ids ...uint64) error {
	for _, id := range ids {
		if id < 1 {
			return status.Error(codes.InvalidArgument, "id must be positive")
		}
	}
	return nil
}
func intID(id uint64) (int64, error) {
	if id == 0 || id > math.MaxInt64 {
		return 0, status.Error(codes.InvalidArgument, "id is out of range")
	}
	return int64(id), nil
}
func operator(value string) error {
	if !limit.Operator(value).Valid() {
		return status.Error(codes.InvalidArgument, "operator is invalid")
	}
	return nil
}
func ts(v *timestamppb.Timestamp) (*time.Time, error) {
	if v == nil {
		return nil, nil
	}
	if err := v.CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid timestamp")
	}
	t := v.AsTime()
	return &t, nil
}
func outTS(v time.Time) *timestamppb.Timestamp { return timestamppb.New(v) }
func outOptionalTS(v *time.Time) *timestamppb.Timestamp {
	if v == nil {
		return nil
	}
	return outTS(*v)
}
func outUnit(v limit.UnitResponse) *pb.Unit {
	return &pb.Unit{Id: uint64(v.ID), Code: v.Code, Active: v.Active, CreatedAt: outTS(v.CreatedAt), UpdatedAt: outOptionalTS(v.UpdatedAt)}
}
func outType(v limit.LimitTypeResponse) *pb.LimitType {
	return &pb.LimitType{Id: uint64(v.ID), Code: v.Code, Description: v.Description, UnitId: uint64(v.UnitID), Active: v.Active, CreatedAt: outTS(v.CreatedAt), UpdatedAt: outOptionalTS(v.UpdatedAt)}
}
func outGroup(v limit.LimitGroupResponse) *pb.LimitGroup {
	return &pb.LimitGroup{Id: uint64(v.ID), WalletType: v.WalletType, Currency: v.Currency, Tier: v.Tier, CreatedAt: outTS(v.CreatedAt), UpdatedAt: outOptionalTS(v.UpdatedAt)}
}
func outValue(v limit.LimitValueResponse) *pb.LimitValue {
	return &pb.LimitValue{Id: uint64(v.ID), LimitGroupId: uint64(v.LimitGroupID), LimitTypeId: uint64(v.LimitTypeID), TransactionType: v.TransactionType, Channel: v.Channel, Operator: string(v.Operator), Value: v.Value.String(), CreatedAt: outTS(v.CreatedAt), UpdatedAt: outOptionalTS(v.UpdatedAt)}
}
func outUser(v limit.UserLimitResponse) *pb.UserLimit {
	return &pb.UserLimit{Id: uint64(v.ID), WalletId: v.WalletID, LimitTypeId: uint64(v.LimitTypeID), TransactionType: v.TransactionType, Channel: v.Channel, Operator: string(v.Operator), Value: v.Value.String(), EffectiveFrom: outTS(v.EffectiveFrom), EffectiveTo: outOptionalTS(v.EffectiveTo), Reason: v.Reason, CreatedBy: v.CreatedBy, CreatedAt: outTS(v.CreatedAt), UpdatedAt: outOptionalTS(v.UpdatedAt)}
}

func (h *limitGRPCHandler) GetLimitConfig(ctx context.Context, r *pb.GetLimitConfigRequest) (*pb.GetLimitConfigResponse, error) {
	if e := require(map[string]string{"walletType": r.WalletType, "currency": r.Currency, "tier": r.Tier, "walletId": r.WalletId}); e != nil {
		return nil, e
	}
	if _, e := currency.ParseISO(r.Currency); e != nil {
		return nil, status.Error(codes.InvalidArgument, "currency must be a valid ISO 4217 currency")
	}
	v, e := h.service.GetLimitConfig(ctx, limit.GetLimitConfigRequest{WalletType: r.WalletType, Currency: r.Currency, Tier: r.Tier, WalletID: r.WalletId, TransactionType: r.TransactionType})
	if e != nil {
		return nil, grpcError(e)
	}
	out := &pb.GetLimitConfigResponse{Rules: make([]*pb.LimitRule, len(v.Rules))}
	for i, x := range v.Rules {
		out.Rules[i] = &pb.LimitRule{LimitType: x.LimitType, Unit: x.Unit, TransactionType: x.TransactionType, Channel: x.Channel, Operator: string(x.Operator), Value: x.Value.String(), Source: string(x.Source)}
	}
	return out, nil
}
func (h *limitGRPCHandler) CreateUnit(ctx context.Context, r *pb.CreateUnitRequest) (*pb.Unit, error) {
	if e := require(map[string]string{"code": r.Code, "updatedBy": r.UpdatedBy}); e != nil {
		return nil, e
	}
	v, e := h.service.CreateUnit(ctx, limit.CreateUnitRequest{Code: r.Code, Active: r.Active, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outUnit(v), nil
}
func (h *limitGRPCHandler) GetUnit(ctx context.Context, r *pb.IdRequest) (*pb.Unit, error) {
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.GetUnit(ctx, id)
	if e != nil {
		return nil, grpcError(e)
	}
	return outUnit(v), nil
}
func (h *limitGRPCHandler) ListUnits(ctx context.Context, r *pb.PageRequest) (*pb.UnitPage, error) {
	p, e := page(r.Page, r.Limit)
	if e != nil {
		return nil, e
	}
	v, e := h.service.ListUnits(ctx, p)
	if e != nil {
		return nil, grpcError(e)
	}
	out := &pb.UnitPage{Items: make([]*pb.Unit, len(v.Items)), Page: v.Page, Limit: v.Limit, Total: v.Total}
	for i, x := range v.Items {
		out.Items[i] = outUnit(x)
	}
	return out, nil
}
func (h *limitGRPCHandler) UpdateUnit(ctx context.Context, r *pb.UpdateUnitRequest) (*pb.Unit, error) {
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	if r.Active == nil {
		return nil, status.Error(codes.InvalidArgument, "active is required")
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.UpdateUnit(ctx, id, limit.UpdateUnitRequest{Active: r.Active, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outUnit(v), nil
}
func (h *limitGRPCHandler) DeleteUnit(ctx context.Context, r *pb.DeleteByIdRequest) (*pb.DeleteResponse, error) {
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	if e := h.service.DeleteUnit(ctx, id, limit.ActorRequest{UpdatedBy: r.UpdatedBy}); e != nil {
		return nil, grpcError(e)
	}
	return &pb.DeleteResponse{}, nil
}

func (h *limitGRPCHandler) CreateLimitType(ctx context.Context, r *pb.CreateLimitTypeRequest) (*pb.LimitType, error) {
	if e := require(map[string]string{"code": r.Code, "updatedBy": r.UpdatedBy}); e != nil {
		return nil, e
	}
	if e := positive(r.UnitId); e != nil {
		return nil, e
	}
	unitID, e := intID(r.UnitId)
	if e != nil {
		return nil, e
	}
	v, e := h.service.CreateLimitType(ctx, limit.CreateLimitTypeRequest{Code: r.Code, Description: r.Description, UnitID: unitID, Active: r.Active, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outType(v), nil
}
func (h *limitGRPCHandler) GetLimitType(ctx context.Context, r *pb.IdRequest) (*pb.LimitType, error) {
	if e := positive(r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.GetLimitType(ctx, id)
	if e != nil {
		return nil, grpcError(e)
	}
	return outType(v), nil
}
func (h *limitGRPCHandler) ListLimitTypes(ctx context.Context, r *pb.PageRequest) (*pb.LimitTypePage, error) {
	p, e := page(r.Page, r.Limit)
	if e != nil {
		return nil, e
	}
	v, e := h.service.ListLimitTypes(ctx, p)
	if e != nil {
		return nil, grpcError(e)
	}
	out := &pb.LimitTypePage{Items: make([]*pb.LimitType, len(v.Items)), Page: v.Page, Limit: v.Limit, Total: v.Total}
	for i, x := range v.Items {
		out.Items[i] = outType(x)
	}
	return out, nil
}
func (h *limitGRPCHandler) UpdateLimitType(ctx context.Context, r *pb.UpdateLimitTypeRequest) (*pb.LimitType, error) {
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	if r.Active == nil {
		return nil, status.Error(codes.InvalidArgument, "active is required")
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.UpdateLimitType(ctx, id, limit.UpdateLimitTypeRequest{Description: r.Description, Active: r.Active, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outType(v), nil
}
func (h *limitGRPCHandler) DeleteLimitType(ctx context.Context, r *pb.DeleteByIdRequest) (*pb.DeleteResponse, error) {
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	if e := h.service.DeleteLimitType(ctx, id, limit.ActorRequest{UpdatedBy: r.UpdatedBy}); e != nil {
		return nil, grpcError(e)
	}
	return &pb.DeleteResponse{}, nil
}

func (h *limitGRPCHandler) CreateLimitGroup(ctx context.Context, r *pb.CreateLimitGroupRequest) (*pb.LimitGroup, error) {
	if e := require(map[string]string{"walletType": r.WalletType, "currency": r.Currency, "tier": r.Tier, "updatedBy": r.UpdatedBy}); e != nil {
		return nil, e
	}
	if _, e := currency.ParseISO(r.Currency); e != nil {
		return nil, status.Error(codes.InvalidArgument, "currency must be a valid ISO 4217 currency")
	}
	v, e := h.service.CreateLimitGroup(ctx, limit.CreateLimitGroupRequest{WalletType: r.WalletType, Currency: r.Currency, Tier: r.Tier, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outGroup(v), nil
}
func (h *limitGRPCHandler) GetLimitGroup(ctx context.Context, r *pb.IdRequest) (*pb.LimitGroup, error) {
	if e := positive(r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.GetLimitGroup(ctx, id)
	if e != nil {
		return nil, grpcError(e)
	}
	return outGroup(v), nil
}
func (h *limitGRPCHandler) DeleteLimitGroup(ctx context.Context, r *pb.DeleteByIdRequest) (*pb.DeleteResponse, error) {
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	if e := h.service.DeleteLimitGroup(ctx, id, limit.ActorRequest{UpdatedBy: r.UpdatedBy}); e != nil {
		return nil, grpcError(e)
	}
	return &pb.DeleteResponse{}, nil
}

func (h *limitGRPCHandler) CreateLimitValue(ctx context.Context, r *pb.CreateLimitValueRequest) (*pb.LimitValue, error) {
	if e := mutation(r.UpdatedBy, r.GroupId, r.LimitTypeId); e != nil {
		return nil, e
	}
	if e := operator(r.Operator); e != nil {
		return nil, e
	}
	groupID, e := intID(r.GroupId)
	if e != nil {
		return nil, e
	}
	typeID, e := intID(r.LimitTypeId)
	if e != nil {
		return nil, e
	}
	v, e := dec(r.Value)
	if e != nil {
		return nil, e
	}
	out, e := h.service.CreateLimitValue(ctx, groupID, limit.CreateLimitValueRequest{LimitTypeID: typeID, TransactionType: r.TransactionType, Channel: r.Channel, Operator: limit.Operator(r.Operator), Value: &v, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outValue(out), nil
}
func (h *limitGRPCHandler) GetLimitValue(ctx context.Context, r *pb.ScopedIdRequest) (*pb.LimitValue, error) {
	if e := positive(r.ParentId, r.Id); e != nil {
		return nil, e
	}
	parentID, e := intID(r.ParentId)
	if e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.GetLimitValue(ctx, parentID, id)
	if e != nil {
		return nil, grpcError(e)
	}
	return outValue(v), nil
}
func (h *limitGRPCHandler) ListLimitValues(ctx context.Context, r *pb.ScopedPageRequest) (*pb.LimitValuePage, error) {
	if e := positive(r.ParentId); e != nil {
		return nil, e
	}
	p, e := page(r.Page, r.Limit)
	if e != nil {
		return nil, e
	}
	parentID, e := intID(r.ParentId)
	if e != nil {
		return nil, e
	}
	v, e := h.service.ListLimitValues(ctx, parentID, p)
	if e != nil {
		return nil, grpcError(e)
	}
	out := &pb.LimitValuePage{Items: make([]*pb.LimitValue, len(v.Items)), Page: v.Page, Limit: v.Limit, Total: v.Total}
	for i, x := range v.Items {
		out.Items[i] = outValue(x)
	}
	return out, nil
}
func (h *limitGRPCHandler) UpdateLimitValue(ctx context.Context, r *pb.UpdateLimitValueRequest) (*pb.LimitValue, error) {
	if e := mutation(r.UpdatedBy, r.GroupId, r.Id); e != nil {
		return nil, e
	}
	if e := operator(r.Operator); e != nil {
		return nil, e
	}
	groupID, e := intID(r.GroupId)
	if e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := dec(r.Value)
	if e != nil {
		return nil, e
	}
	out, e := h.service.UpdateLimitValue(ctx, groupID, id, limit.UpdateLimitValueRequest{Operator: limit.Operator(r.Operator), Value: &v, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outValue(out), nil
}
func (h *limitGRPCHandler) DeleteLimitValue(ctx context.Context, r *pb.DeleteScopedRequest) (*pb.DeleteResponse, error) {
	if e := mutation(r.UpdatedBy, r.ParentId, r.Id); e != nil {
		return nil, e
	}
	parentID, e := intID(r.ParentId)
	if e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	if e := h.service.DeleteLimitValue(ctx, parentID, id, limit.ActorRequest{UpdatedBy: r.UpdatedBy}); e != nil {
		return nil, grpcError(e)
	}
	return &pb.DeleteResponse{}, nil
}

func (h *limitGRPCHandler) CreateUserLimit(ctx context.Context, r *pb.CreateUserLimitRequest) (*pb.UserLimit, error) {
	if e := require(map[string]string{"walletId": r.WalletId, "updatedBy": r.UpdatedBy}); e != nil {
		return nil, e
	}
	if e := mutation(r.UpdatedBy, r.LimitTypeId); e != nil {
		return nil, e
	}
	if e := operator(r.Operator); e != nil {
		return nil, e
	}
	typeID, e := intID(r.LimitTypeId)
	if e != nil {
		return nil, e
	}
	v, e := dec(r.Value)
	if e != nil {
		return nil, e
	}
	from, e := ts(r.EffectiveFrom)
	if e != nil {
		return nil, e
	}
	to, e := ts(r.EffectiveTo)
	if e != nil {
		return nil, e
	}
	out, e := h.service.CreateUserLimit(ctx, r.WalletId, limit.CreateUserLimitRequest{LimitTypeID: typeID, TransactionType: r.TransactionType, Channel: r.Channel, Operator: limit.Operator(r.Operator), Value: &v, EffectiveFrom: from, EffectiveTo: to, Reason: r.Reason, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outUser(out), nil
}
func (h *limitGRPCHandler) GetUserLimit(ctx context.Context, r *pb.WalletIdRequest) (*pb.UserLimit, error) {
	if e := require(map[string]string{"walletId": r.WalletId}); e != nil {
		return nil, e
	}
	if e := positive(r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	v, e := h.service.GetUserLimit(ctx, r.WalletId, id)
	if e != nil {
		return nil, grpcError(e)
	}
	return outUser(v), nil
}
func (h *limitGRPCHandler) ListUserLimits(ctx context.Context, r *pb.WalletPageRequest) (*pb.UserLimitPage, error) {
	if e := require(map[string]string{"walletId": r.WalletId}); e != nil {
		return nil, e
	}
	p, e := page(r.Page, r.Limit)
	if e != nil {
		return nil, e
	}
	v, e := h.service.ListUserLimits(ctx, r.WalletId, p)
	if e != nil {
		return nil, grpcError(e)
	}
	out := &pb.UserLimitPage{Items: make([]*pb.UserLimit, len(v.Items)), Page: v.Page, Limit: v.Limit, Total: v.Total}
	for i, x := range v.Items {
		out.Items[i] = outUser(x)
	}
	return out, nil
}
func (h *limitGRPCHandler) UpdateUserLimit(ctx context.Context, r *pb.UpdateUserLimitRequest) (*pb.UserLimit, error) {
	if e := require(map[string]string{"walletId": r.WalletId, "updatedBy": r.UpdatedBy}); e != nil {
		return nil, e
	}
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	if e := operator(r.Operator); e != nil {
		return nil, e
	}
	v, e := dec(r.Value)
	if e != nil {
		return nil, e
	}
	from, e := ts(r.EffectiveFrom)
	if e != nil || from == nil {
		if e == nil {
			e = status.Error(codes.InvalidArgument, "effectiveFrom is required")
		}
		return nil, e
	}
	to, e := ts(r.EffectiveTo)
	if e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	out, e := h.service.UpdateUserLimit(ctx, r.WalletId, id, limit.UpdateUserLimitRequest{Operator: limit.Operator(r.Operator), Value: &v, EffectiveFrom: from, EffectiveTo: to, Reason: r.Reason, UpdatedBy: r.UpdatedBy})
	if e != nil {
		return nil, grpcError(e)
	}
	return outUser(out), nil
}
func (h *limitGRPCHandler) DeleteUserLimit(ctx context.Context, r *pb.DeleteUserLimitRequest) (*pb.DeleteResponse, error) {
	if e := require(map[string]string{"walletId": r.WalletId, "updatedBy": r.UpdatedBy}); e != nil {
		return nil, e
	}
	if e := mutation(r.UpdatedBy, r.Id); e != nil {
		return nil, e
	}
	id, e := intID(r.Id)
	if e != nil {
		return nil, e
	}
	if e := h.service.DeleteUserLimit(ctx, r.WalletId, id, limit.ActorRequest{UpdatedBy: r.UpdatedBy}); e != nil {
		return nil, grpcError(e)
	}
	return &pb.DeleteResponse{}, nil
}

var _ pb.LimitServiceServer = (*limitGRPCHandler)(nil)
