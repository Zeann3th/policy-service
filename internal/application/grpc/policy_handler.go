package grpc

import (
	"context"

	"github.com/shopspring/decimal"
	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		result.Rules[i] = &pb.PolicyRule{PolicyType: value.PolicyType, Kind: string(value.Kind), Unit: value.Unit, Base: string(value.Base), TransactionType: value.TransactionType, Channel: value.Channel, Direction: string(value.Direction), WindowType: string(value.WindowType), Operator: string(value.Operator), Value: value.Value.String(), Cap: outOptionalDec(value.Cap), Source: string(value.Source)}
	}
	return result, nil
}

func (h *policyHandler) CreatePolicy(ctx context.Context, request *pb.CreatePolicyRequest) (*pb.Policy, error) {
	if err := mutation(request.UpdatedBy, request.GroupId); err != nil {
		return nil, err
	}
	fields, err := inPolicyFields(request)
	if err != nil {
		return nil, err
	}
	groupID, _ := intID(request.GroupId)
	response, err := h.service.CreatePolicy(ctx, groupID, policy.CreatePolicyRequest{PolicyFields: fields, UpdatedBy: request.UpdatedBy})
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
	change, err := inPolicyChange(request)
	if err != nil {
		return nil, err
	}
	groupID, _ := intID(request.GroupId)
	id, _ := intID(request.Id)
	response, err := h.service.UpdatePolicy(ctx, groupID, id, policy.UpdatePolicyRequest{PolicyChange: change, UpdatedBy: request.UpdatedBy})
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
	if err := mutation(request.UpdatedBy); err != nil {
		return nil, err
	}
	fields, err := inPolicyFields(request)
	if err != nil {
		return nil, err
	}
	response, err := h.service.CreateCustomPolicy(ctx, request.WalletId, policy.CreateCustomPolicyRequest{PolicyFields: fields, Reason: request.Reason, UpdatedBy: request.UpdatedBy})
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
	change, err := inPolicyChange(request)
	if err != nil {
		return nil, err
	}
	id, _ := intID(request.Id)
	response, err := h.service.UpdateCustomPolicy(ctx, request.WalletId, id, policy.UpdateCustomPolicyRequest{PolicyChange: change, Reason: request.Reason, UpdatedBy: request.UpdatedBy})
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

// policyChangeMessage is what every proto request that changes a policy
// carries (system or custom, create or update).
type policyChangeMessage interface {
	GetBase() string
	GetOperator() string
	GetValue() string
	GetCap() string
	GetEffectiveFrom() *timestamppb.Timestamp
	GetEffectiveTo() *timestamppb.Timestamp
}

// policyFieldsMessage is what a proto request creating a policy carries.
type policyFieldsMessage interface {
	policyChangeMessage
	GetPolicyTypeId() uint64
	GetTransactionType() string
	GetChannel() string
	GetDirection() string
	GetWindowType() string
	GetUnitId() uint64
}

func inPolicyChange(message policyChangeMessage) (policy.PolicyChange, error) {
	if err := operator(message.GetOperator()); err != nil {
		return policy.PolicyChange{}, err
	}
	if err := base(message.GetBase()); err != nil {
		return policy.PolicyChange{}, err
	}
	value, err := dec(message.GetValue())
	if err != nil {
		return policy.PolicyChange{}, err
	}
	var capValue *decimal.Decimal
	if message.GetCap() != "" {
		parsed, err := dec(message.GetCap())
		if err != nil {
			return policy.PolicyChange{}, err
		}
		capValue = &parsed
	}
	from, err := ts(message.GetEffectiveFrom())
	if err != nil {
		return policy.PolicyChange{}, err
	}
	to, err := ts(message.GetEffectiveTo())
	if err != nil {
		return policy.PolicyChange{}, err
	}
	return policy.PolicyChange{Base: policy.Base(message.GetBase()), Operator: policy.Operator(message.GetOperator()), Value: &value, Cap: capValue, EffectiveFrom: from, EffectiveTo: to}, nil
}

func inPolicyFields(message policyFieldsMessage) (policy.PolicyFields, error) {
	typeID, err := intID(message.GetPolicyTypeId())
	if err != nil {
		return policy.PolicyFields{}, err
	}
	if err := direction(message.GetDirection()); err != nil {
		return policy.PolicyFields{}, err
	}
	if err := windowType(message.GetWindowType()); err != nil {
		return policy.PolicyFields{}, err
	}
	var unitID *int64
	if message.GetUnitId() != 0 {
		id, err := intID(message.GetUnitId())
		if err != nil {
			return policy.PolicyFields{}, err
		}
		unitID = &id
	}
	change, err := inPolicyChange(message)
	if err != nil {
		return policy.PolicyFields{}, err
	}
	return policy.PolicyFields{
		PolicyTypeID:    typeID,
		TransactionType: message.GetTransactionType(),
		Channel:         message.GetChannel(),
		Direction:       policy.Direction(message.GetDirection()),
		WindowType:      policy.WindowType(message.GetWindowType()),
		UnitID:          unitID,
		PolicyChange:    change,
	}, nil
}
