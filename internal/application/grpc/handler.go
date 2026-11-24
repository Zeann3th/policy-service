package grpc

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
	"golang.org/x/text/currency"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func grpcError(err error) error {
	var value *status.Status
	switch {
	case errors.Is(err, core.ErrBadRequest):
		value = status.New(codes.InvalidArgument, err.Error())
	case errors.Is(err, core.ErrNotFound):
		value = status.New(codes.NotFound, core.ErrNotFound.Message)
	case errors.Is(err, core.ErrConflict):
		value = status.New(codes.AlreadyExists, core.ErrConflict.Message)
	default:
		value = status.New(codes.Internal, core.ErrInternal.Message)
	}
	return &statusError{status: value, cause: err}
}

func require(fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return status.Errorf(codes.InvalidArgument, "%s is required", name)
		}
	}
	return nil
}

func page(page, limit int32) (core.PageRequest, error) {
	if page < 0 || page > core.MaxPage || limit < 0 || limit > 100 {
		return core.PageRequest{}, status.Error(codes.InvalidArgument, "page must be positive and limit must be between 1 and 100")
	}
	return core.PageRequest{Page: page, Limit: limit}, nil
}

func dec(value string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return parsed, status.Error(codes.InvalidArgument, "value must be a decimal")
	}
	return parsed, nil
}

func mutation(actor string, ids ...uint64) error {
	if strings.TrimSpace(actor) == "" {
		return status.Error(codes.InvalidArgument, "updatedBy is required")
	}
	return positive(ids...)
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

// enum checks an optional enum field: empty, or a value its type knows.
func enum[T interface {
	~string
	Valid() bool
}](name, value string) error {
	if value != "" && !T(value).Valid() {
		return status.Errorf(codes.InvalidArgument, "%s is invalid", name)
	}
	return nil
}

// firstError is the first non-nil error, for checks that cannot fail
// halfway (the fields are already parsed).
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func isoCurrency(value string) error {
	if _, err := currency.ParseISO(value); err != nil {
		return status.Error(codes.InvalidArgument, "currency must be a valid ISO 4217 currency")
	}
	return nil
}

func outOptionalID(value *int64) *uint64 {
	if value == nil {
		return nil
	}
	id := uint64(*value)
	return &id
}

func outOptionalDec(value *decimal.Decimal) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

func ts(value *timestamppb.Timestamp) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	if err := value.CheckValid(); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid timestamp")
	}
	timestamp := value.AsTime()
	return &timestamp, nil
}

func outTS(value time.Time) *timestamppb.Timestamp {
	return timestamppb.New(value)
}

func outOptionalTS(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return outTS(*value)
}

func outUnit(value unit.Response) *pb.Unit {
	return &pb.Unit{Id: uint64(value.ID), Code: value.Code, Description: value.Description, Active: value.Active, CreatedAt: outTS(value.CreatedAt), UpdatedAt: outOptionalTS(value.UpdatedAt)}
}

func outPolicyType(value policy.PolicyTypeResponse) *pb.PolicyType {
	return &pb.PolicyType{Id: uint64(value.ID), Code: value.Code, Description: value.Description, Kind: string(value.Kind), Active: value.Active, CreatedAt: outTS(value.CreatedAt), UpdatedAt: outOptionalTS(value.UpdatedAt)}
}

func outPolicyGroup(value policy.PolicyGroupResponse) *pb.PolicyGroup {
	return &pb.PolicyGroup{Id: uint64(value.ID), WalletType: value.WalletType, Currency: value.Currency, Tier: value.Tier, CreatedAt: outTS(value.CreatedAt), UpdatedAt: outOptionalTS(value.UpdatedAt)}
}

func outPolicy(value policy.PolicyResponse) *pb.Policy {
	return &pb.Policy{Id: uint64(value.ID), PolicyGroupId: uint64(value.PolicyGroupID), PolicyTypeId: uint64(value.PolicyTypeID), TransactionType: value.TransactionType, Channel: value.Channel, Direction: string(value.Direction), WindowType: string(value.WindowType), IsEnforced: value.IsEnforced, UnitId: outOptionalID(value.UnitID), Base: string(value.Base), Operator: string(value.Operator), Value: value.Value.String(), Cap: outOptionalDec(value.Cap), EffectiveFrom: outTS(value.EffectiveFrom), EffectiveTo: outOptionalTS(value.EffectiveTo), CreatedAt: outTS(value.CreatedAt), UpdatedAt: outOptionalTS(value.UpdatedAt)}
}

func outCustomPolicy(value policy.CustomPolicyResponse) *pb.CustomPolicy {
	return &pb.CustomPolicy{Id: uint64(value.ID), WalletId: value.WalletID, PolicyTypeId: uint64(value.PolicyTypeID), TransactionType: value.TransactionType, Channel: value.Channel, Direction: string(value.Direction), WindowType: string(value.WindowType), IsEnforced: value.IsEnforced, UnitId: outOptionalID(value.UnitID), Base: string(value.Base), Operator: string(value.Operator), Value: value.Value.String(), Cap: outOptionalDec(value.Cap), EffectiveFrom: outTS(value.EffectiveFrom), EffectiveTo: outOptionalTS(value.EffectiveTo), Reason: value.Reason, CreatedBy: value.CreatedBy, CreatedAt: outTS(value.CreatedAt), UpdatedAt: outOptionalTS(value.UpdatedAt)}
}
