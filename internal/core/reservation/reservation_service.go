package reservation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

type Service interface {
	Reserve(context.Context, ReserveRequest) (ReserveResponse, error)
	Release(ctx context.Context, requestID string) (ReleaseResponse, error)
}

type service struct {
	store    Store
	policies PolicyResolver
}

func NewService(store Store, policies PolicyResolver) Service {
	return &service{store: store, policies: policies}
}

// Reserve evaluates a transaction against both wallets' policies and, when
// nothing is violated, adds it to their usage.
//
//  1. Validate the request and resolve each side's policies (outside any
//     transaction); a side without policies is a bad request.
//  2. Insert the reservation as PROCESSING; a duplicate request id fails.
//  3. In one transaction holding a lock per wallet: read usage, evaluate,
//     then either mark it REJECTED or record the ledger, add usage and mark
//     it RESERVED, storing the response as the result.
//  4. If that transaction fails it is rolled back and the reservation is
//     marked FAILED before the error is returned.
func (s *service) Reserve(ctx context.Context, request ReserveRequest) (ReserveResponse, error) {
	if err := validate(&request); err != nil {
		return ReserveResponse{}, err
	}
	senderPolicies, err := s.resolve(ctx, request, request.Sender, policy.DirectionOut, "sender")
	if err != nil {
		return ReserveResponse{}, err
	}
	receiverPolicies, err := s.resolve(ctx, request, request.Receiver, policy.DirectionIn, "receiver")
	if err != nil {
		return ReserveResponse{}, err
	}

	id, err := uuid.NewV7()
	if err != nil {
		return ReserveResponse{}, fmt.Errorf("generate reservation id: %w", err)
	}
	value, err := s.store.Create(ctx, Reservation{
		ID:              id,
		RequestID:       request.RequestID,
		Status:          StatusProcessing,
		TransactionType: request.TransactionType,
		Channel:         request.Channel,
		Currency:        request.Currency,
		Amount:          *request.Amount,
		Sender:          party(request.Sender),
		Receiver:        party(request.Receiver),
	})
	if err != nil {
		return ReserveResponse{}, err
	}

	response, err := s.reserve(ctx, value, senderPolicies, receiverPolicies)
	if err != nil {
		// the caller may be gone; the reservation must not stay PROCESSING
		failErr := s.store.Fail(context.WithoutCancel(ctx), value.ID, value.CreatedAt)
		return ReserveResponse{}, errors.Join(err, failErr)
	}
	return response, nil
}

// Release reverses a RESERVED reservation, found by its original request id.
//
//  1. Find the reservation (outside any transaction); only RESERVED can be
//     released.
//  2. In one transaction holding the same wallet locks as Reserve: move it
//     to RELEASED (guarding against a concurrent release), read its RESERVE
//     ledger entries and apply each one negated as a RELEASE entry, to the
//     same usage window it was reserved in.
//  3. On failure the transaction rolls back and it stays RESERVED.
func (s *service) Release(ctx context.Context, requestID string) (ReleaseResponse, error) {
	if strings.TrimSpace(requestID) == "" {
		return ReleaseResponse{}, core.ErrBadRequest.WithMessage("requestId is required")
	}
	value, err := s.store.Find(ctx, requestID)
	if err != nil {
		return ReleaseResponse{}, err
	}
	if value.Status != StatusReserved {
		return ReleaseResponse{}, notReserved(value.Status)
	}

	err = s.store.Transact(ctx, []string{value.Sender.WalletID, value.Receiver.WalletID}, func(tx Tx) error {
		if err := tx.Transition(ctx, value.ID, value.CreatedAt, StatusReserved, StatusReleased); err != nil {
			return err
		}
		entries, err := tx.Entries(ctx, value.ID, value.CreatedAt, EntryReserve)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			entry.EntryType, entry.Delta = EntryRelease, entry.Delta.Neg()
			if _, err := tx.Apply(ctx, entry); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return ReleaseResponse{}, err
	}
	return ReleaseResponse{ReservationID: value.ID, RequestID: value.RequestID, Status: StatusReleased, ResponseTime: time.Now()}, nil
}

func notReserved(status Status) error {
	return core.ErrConflict.WithMessage(fmt.Sprintf("reservation is %s, only %s can be released", status, StatusReserved))
}

func validate(request *ReserveRequest) error {
	if request.Amount == nil || !request.Amount.IsPositive() {
		return core.ErrBadRequest.WithMessage("amount must be positive")
	}
	if request.Amount.Exponent() < -moneyPlaces && !request.Amount.Equal(request.Amount.Round(moneyPlaces)) {
		return core.ErrBadRequest.WithMessage(fmt.Sprintf("amount supports at most %d decimal places", moneyPlaces))
	}
	if strings.TrimSpace(request.RequestID) == "" {
		return core.ErrBadRequest.WithMessage("requestId is required")
	}
	if request.Sender.WalletID == request.Receiver.WalletID {
		return core.ErrBadRequest.WithMessage("sender and receiver must be different wallets")
	}
	if request.Channel == "" {
		request.Channel = policy.All
	}
	return nil
}

func party(request PartyRequest) Party {
	return Party{WalletID: request.WalletID, WalletType: request.WalletType, Tier: request.Tier}
}

func (s *service) resolve(ctx context.Context, request ReserveRequest, side PartyRequest, direction policy.Direction, name string) ([]policy.Policy, error) {
	policies, err := s.policies.Resolve(ctx, policy.ResolveRequest{
		WalletID:        side.WalletID,
		WalletType:      side.WalletType,
		Currency:        request.Currency,
		Tier:            side.Tier,
		TransactionType: request.TransactionType,
		Channel:         request.Channel,
		Direction:       direction,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve %s policies: %w", name, err)
	}
	if len(policies) == 0 {
		return nil, core.ErrBadRequest.WithMessage(name + " has no applicable policy")
	}
	return policies, nil
}

func (s *service) reserve(ctx context.Context, value Reservation, senderPolicies, receiverPolicies []policy.Policy) (ReserveResponse, error) {
	var response ReserveResponse
	err := s.store.Transact(ctx, []string{value.Sender.WalletID, value.Receiver.WalletID}, func(tx Tx) error {
		sender, senderEntries, err := evaluateSide(ctx, tx, value, value.Sender.WalletID, senderPolicies)
		if err != nil {
			return err
		}
		receiver, receiverEntries, err := evaluateSide(ctx, tx, value, value.Receiver.WalletID, receiverPolicies)
		if err != nil {
			return err
		}

		response = ReserveResponse{ReservationID: value.ID, RequestID: value.RequestID, Status: StatusReserved, Sender: sender, Receiver: receiver}
		if sender.IsViolated || receiver.IsViolated {
			response.Status = StatusRejected
		} else {
			for _, entry := range append(senderEntries, receiverEntries...) {
				entry.ReservationID, entry.ReservationCreatedAt = value.ID, value.CreatedAt
				if _, err := tx.Apply(ctx, entry); err != nil {
					return err
				}
			}
		}

		response.ResponseTime = time.Now()
		result, err := json.Marshal(response)
		if err != nil {
			return fmt.Errorf("encode reservation result: %w", err)
		}
		return tx.Finish(ctx, value.ID, value.CreatedAt, response.Status, result)
	})
	return response, err
}

func evaluateSide(ctx context.Context, tx Tx, value Reservation, walletID string, policies []policy.Policy) (PartyResult, []LedgerEntry, error) {
	usage := make(map[UsageKey]decimal.Decimal)
	for _, p := range policies {
		if !tracked(p) {
			continue
		}
		key := usageKey(walletID, p, value.CreatedAt)
		current, err := tx.Usage(ctx, key)
		if err != nil {
			return PartyResult{}, nil, err
		}
		usage[key] = current
	}
	return evaluate(walletID, value.Amount, policies, usage, value.CreatedAt)
}
