package grpc

import (
	"context"

	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core/reservation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type reservationHandler struct {
	pb.UnimplementedReservationServiceServer
	service reservation.Service
}

func (h *reservationHandler) Reserve(ctx context.Context, request *pb.ReserveRequest) (*pb.ReserveResponse, error) {
	if request.Sender == nil || request.Receiver == nil {
		return nil, status.Error(codes.InvalidArgument, "sender and receiver are required")
	}
	if err := require(map[string]string{
		"requestId": request.RequestId, "transactionType": request.TransactionType, "currency": request.Currency, "amount": request.Amount,
		"sender.walletId": request.Sender.WalletId, "sender.walletType": request.Sender.WalletType, "sender.tier": request.Sender.Tier,
		"receiver.walletId": request.Receiver.WalletId, "receiver.walletType": request.Receiver.WalletType, "receiver.tier": request.Receiver.Tier,
	}); err != nil {
		return nil, err
	}
	if err := isoCurrency(request.Currency); err != nil {
		return nil, err
	}
	amount, err := dec(request.Amount)
	if err != nil {
		return nil, err
	}
	response, err := h.service.Reserve(ctx, reservation.ReserveRequest{
		RequestID:       request.RequestId,
		TransactionType: request.TransactionType,
		Channel:         request.Channel,
		Currency:        request.Currency,
		Amount:          &amount,
		Sender:          inParty(request.Sender),
		Receiver:        inParty(request.Receiver),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &pb.ReserveResponse{
		ReservationId: response.ReservationID.String(),
		RequestId:     response.RequestID,
		Status:        string(response.Status),
		Sender:        outPartyResult(response.Sender),
		Receiver:      outPartyResult(response.Receiver),
		ResponseTime:  outTS(response.ResponseTime),
	}, nil
}

func (h *reservationHandler) Release(ctx context.Context, request *pb.ReleaseRequest) (*pb.ReleaseResponse, error) {
	if err := require(map[string]string{"requestId": request.RequestId}); err != nil {
		return nil, err
	}
	response, err := h.service.Release(ctx, request.RequestId)
	if err != nil {
		return nil, grpcError(err)
	}
	return &pb.ReleaseResponse{
		ReservationId: response.ReservationID.String(),
		RequestId:     response.RequestID,
		Status:        string(response.Status),
		ResponseTime:  outTS(response.ResponseTime),
	}, nil
}

func inParty(value *pb.ReservationParty) reservation.PartyRequest {
	return reservation.PartyRequest{WalletID: value.WalletId, WalletType: value.WalletType, Tier: value.Tier}
}

func outPartyResult(value reservation.PartyResult) *pb.PartyResult {
	result := &pb.PartyResult{WalletId: value.WalletID, IsViolated: value.IsViolated, Charges: make([]*pb.Charge, len(value.Charges)), AppliedPolicies: make([]*pb.AppliedPolicy, len(value.AppliedPolicies))}
	for i, charge := range value.Charges {
		result.Charges[i] = &pb.Charge{Role: string(charge.Role), Amount: charge.Amount.String()}
	}
	for i, applied := range value.AppliedPolicies {
		var usage, charge *string
		if applied.Usage != nil {
			value := applied.Usage.String()
			usage = &value
		}
		if applied.Charge != nil {
			value := applied.Charge.String()
			charge = &value
		}
		result.AppliedPolicies[i] = &pb.AppliedPolicy{PolicyId: uint64(applied.PolicyID), PolicyType: applied.PolicyType, Kind: string(applied.Kind), Role: string(applied.Role), Unit: applied.Unit, Base: string(applied.Base), Source: string(applied.Source), TransactionType: applied.TransactionType, Channel: applied.Channel, Direction: string(applied.Direction), WindowType: string(applied.WindowType), WindowStart: applied.WindowStart, Operator: string(applied.Operator), Value: applied.Value.String(), Usage: usage, Charge: charge, Cap: outOptionalDec(applied.Cap), Violated: applied.Violated}
	}
	return result
}
