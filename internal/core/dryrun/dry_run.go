// Package dryrun evaluates a transaction exactly as a reservation would,
// against the resolved policies and the wallets' live usage, and saves
// nothing: no reservation, usage or ledger row is written and no lock is
// taken.
package dryrun

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
)

type Request struct {
	TransactionType string                   `binding:"required" example:"PAYMENT" json:"transactionType"`
	Channel         string                   `example:"WEB" json:"channel"`
	Currency        string                   `binding:"required,iso4217" example:"VND" json:"currency"`
	Amount          *decimal.Decimal         `binding:"required" example:"100000" json:"amount" swaggertype:"string"`
	Sender          reservation.PartyRequest `binding:"required" json:"sender"`
	Receiver        reservation.PartyRequest `binding:"required" json:"receiver"`
	// UsageOverrides replaces the live window total (before this
	// transaction) of a tracked policy, by policy id, per side.
	UsageOverrides Overrides `json:"usageOverrides"`
}

type Overrides struct {
	Sender   map[int64]decimal.Decimal `json:"sender,omitempty" swaggertype:"object,string"`
	Receiver map[int64]decimal.Decimal `json:"receiver,omitempty" swaggertype:"object,string"`
}

type Response struct {
	// Status is what a reservation would answer: REJECTED when either side
	// violates a limit, otherwise RESERVED.
	Status      reservation.Status `enums:"RESERVED,REJECTED" json:"status"`
	Sender      Side               `json:"sender"`
	Receiver    Side               `json:"receiver"`
	EvaluatedAt time.Time          `format:"date-time" json:"evaluatedAt"`
}

// Side is one wallet's result plus the usage it was evaluated against.
type Side struct {
	reservation.PartyResult
	Usage []Usage `json:"usage"`
}

// Usage is a tracked policy's window total before this transaction.
type Usage struct {
	PolicyID    int64           `json:"policyId"`
	WindowStart string          `example:"2026-11-23" json:"windowStart"`
	Live        decimal.Decimal `json:"live" swaggertype:"string"`
	Used        decimal.Decimal `json:"used" swaggertype:"string"`
	Overridden  bool            `json:"overridden"`
}

// UsageReader reads a usage window outside any transaction.
type UsageReader interface {
	Usage(context.Context, reservation.UsageKey) (decimal.Decimal, error)
}

type Service interface {
	Run(context.Context, Request) (Response, error)
}

type service struct {
	policies reservation.PolicyResolver
	usage    UsageReader
}

func NewService(policies reservation.PolicyResolver, usage UsageReader) Service {
	return &service{policies: policies, usage: usage}
}

func (s *service) Run(ctx context.Context, request Request) (Response, error) {
	if err := reservation.ValidateTransaction(request.Amount, request.Sender, request.Receiver); err != nil {
		return Response{}, err
	}
	if request.Channel == "" {
		request.Channel = policy.All
	}
	at := time.Now()
	sender, err := s.side(ctx, request, request.Sender, policy.DirectionOut, "sender", request.UsageOverrides.Sender, at)
	if err != nil {
		return Response{}, err
	}
	receiver, err := s.side(ctx, request, request.Receiver, policy.DirectionIn, "receiver", request.UsageOverrides.Receiver, at)
	if err != nil {
		return Response{}, err
	}
	status := reservation.StatusReserved
	if sender.IsViolated || receiver.IsViolated {
		status = reservation.StatusRejected
	}
	return Response{Status: status, Sender: sender, Receiver: receiver, EvaluatedAt: at}, nil
}

func (s *service) side(ctx context.Context, request Request, party reservation.PartyRequest, direction policy.Direction, name string, overrides map[int64]decimal.Decimal, at time.Time) (Side, error) {
	policies, err := reservation.ResolveParty(ctx, s.policies, policy.ResolveRequest{
		WalletID:        party.WalletID,
		WalletType:      party.WalletType,
		Currency:        request.Currency,
		Tier:            party.Tier,
		TransactionType: request.TransactionType,
		Channel:         request.Channel,
		Direction:       direction,
	}, name)
	if err != nil {
		return Side{}, err
	}
	keys := reservation.UsageKeys(party.WalletID, policies, at)
	usage := make(map[reservation.UsageKey]decimal.Decimal, len(keys))
	lines := make([]Usage, 0, len(keys))
	for _, p := range policies {
		key, ok := keys[p.ID]
		if !ok {
			continue
		}
		live, err := s.usage.Usage(ctx, key)
		if err != nil {
			return Side{}, fmt.Errorf("read %s usage: %w", name, err)
		}
		used, overridden := overrides[p.ID]
		if !overridden {
			used = live
		}
		usage[key] = used
		lines = append(lines, Usage{PolicyID: p.ID, WindowStart: key.WindowStart.Format(time.DateOnly), Live: live, Used: used, Overridden: overridden})
	}
	result, err := reservation.Preview(party.WalletID, *request.Amount, policies, usage, at)
	if err != nil {
		return Side{}, err
	}
	return Side{PartyResult: result, Usage: lines}, nil
}
