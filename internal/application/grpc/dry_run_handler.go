package grpc

import (
	"context"

	"github.com/shopspring/decimal"
	pb "github.com/zeann3th/policy-service/internal/application/grpc/codegen"
	"github.com/zeann3th/policy-service/internal/core/dryrun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type dryRunHandler struct {
	pb.UnimplementedDryRunServiceServer
	service dryrun.Service
}

func (h *dryRunHandler) DryRun(ctx context.Context, request *pb.DryRunRequest) (*pb.DryRunResponse, error) {
	if request.Sender == nil || request.Receiver == nil {
		return nil, status.Error(codes.InvalidArgument, "sender and receiver are required")
	}
	if err := require(map[string]string{
		"transactionType": request.TransactionType, "currency": request.Currency, "amount": request.Amount,
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
	senderUsage, err := inUsage(request.SenderUsage)
	if err != nil {
		return nil, err
	}
	receiverUsage, err := inUsage(request.ReceiverUsage)
	if err != nil {
		return nil, err
	}
	response, err := h.service.Run(ctx, dryrun.Request{
		TransactionType: request.TransactionType,
		Channel:         request.Channel,
		Currency:        request.Currency,
		Amount:          &amount,
		Sender:          inParty(request.Sender),
		Receiver:        inParty(request.Receiver),
		UsageOverrides:  dryrun.Overrides{Sender: senderUsage, Receiver: receiverUsage},
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &pb.DryRunResponse{
		Status:      string(response.Status),
		Sender:      outDryRunSide(response.Sender),
		Receiver:    outDryRunSide(response.Receiver),
		EvaluatedAt: outTS(response.EvaluatedAt),
	}, nil
}

func inUsage(values map[uint64]string) (map[int64]decimal.Decimal, error) {
	result := make(map[int64]decimal.Decimal, len(values))
	for id, raw := range values {
		policyID, err := intID(id)
		if err != nil {
			return nil, err
		}
		value, err := dec(raw)
		if err != nil {
			return nil, err
		}
		result[policyID] = value
	}
	return result, nil
}

func outDryRunSide(value dryrun.Side) *pb.DryRunSide {
	result := &pb.DryRunSide{Result: outPartyResult(value.PartyResult), Usage: make([]*pb.DryRunUsage, len(value.Usage))}
	for i, usage := range value.Usage {
		result.Usage[i] = &pb.DryRunUsage{PolicyId: uint64(usage.PolicyID), WindowStart: usage.WindowStart, Live: usage.Live.String(), Used: usage.Used.String(), Overridden: usage.Overridden}
	}
	return result
}
