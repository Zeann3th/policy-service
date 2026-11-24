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
//  1. Find the reservation (outside any transaction). An already RELEASED
//     one returns the original release response (idempotent); anything else
//     but RESERVED is a conflict.
//  2. In one transaction holding the same wallet locks as Reserve: move it
//     to RELEASED (guarding against a concurrent release), read its RESERVE
//     ledger entries and apply each one negated as a RELEASE entry, to the
//     same usage window it was reserved in.
//  3. A concurrent release that won the race makes this one return the same
//     response. On any other failure the transaction rolls back and it stays
//     RESERVED.
//
// The response time is the release time stored on the reservation
// (updated_at; RELEASED is final), so every call returns identical data.
func (s *service) Release(ctx context.Context, requestID string) (ReleaseResponse, error) {
	if strings.TrimSpace(requestID) == "" {
		return ReleaseResponse{}, core.ErrBadRequest.WithMessage("requestId is required")
	}
	value, err := s.store.Find(ctx, requestID)
	if err != nil {
		return ReleaseResponse{}, err
	}
	if value.Status == StatusReleased {
		return releaseResponse(value), nil
	}
	if value.Status != StatusReserved {
		return ReleaseResponse{}, notReserved(value.Status)
	}

	err = s.store.Transact(ctx, []string{value.Sender.WalletID, value.Receiver.WalletID}, func(tx Tx) error {
		releasedAt, err := tx.Transition(ctx, value.ID, value.CreatedAt, StatusReserved, StatusReleased)
		if err != nil {
			return err
		}
		value.Status, value.UpdatedAt = StatusReleased, &releasedAt
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
	if errors.Is(err, core.ErrConflict) {
		// another release of the same request won the race
		if current, findErr := s.store.Find(ctx, requestID); findErr == nil && current.Status == StatusReleased {
			return releaseResponse(current), nil
		}
	}
	if err != nil {
		return ReleaseResponse{}, err
	}
	return releaseResponse(value), nil
}

func releaseResponse(value Reservation) ReleaseResponse {
	response := ReleaseResponse{ReservationID: value.ID, RequestID: value.RequestID, Status: value.Status}
	if value.UpdatedAt != nil {
		response.ResponseTime = *value.UpdatedAt
	}
	return response
}

func notReserved(status Status) error {
	return core.ErrConflict.WithMessage(fmt.Sprintf("reservation is %s, only %s can be released", status, StatusReserved))
}

func validate(request *ReserveRequest) error {
	if err := validateTransaction(request.Amount, request.Sender, request.Receiver); err != nil {
		return err
	}
	if strings.TrimSpace(request.RequestID) == "" {
		return core.ErrBadRequest.WithMessage("requestId is required")
	}
	if request.Channel == "" {
		request.Channel = policy.All
	}
	return nil
}

// validateTransaction checks the amount and the two sides of a transaction.
func validateTransaction(amount *decimal.Decimal, sender, receiver PartyRequest) error {
	if amount == nil || !amount.IsPositive() {
		return core.ErrBadRequest.WithMessage("amount must be positive")
	}
	if amount.Exponent() < -moneyPlaces && !amount.Equal(amount.Round(moneyPlaces)) {
		return core.ErrBadRequest.WithMessage(fmt.Sprintf("amount supports at most %d decimal places", moneyPlaces))
	}
	if sender.WalletID == receiver.WalletID {
		return core.ErrBadRequest.WithMessage("sender and receiver must be different wallets")
	}
	return nil
}

func party(request PartyRequest) Party {
	return Party{WalletID: request.WalletID, WalletType: request.WalletType, Tier: request.Tier}
}

func (s *service) resolve(ctx context.Context, request ReserveRequest, side PartyRequest, direction policy.Direction, name string) ([]policy.Policy, error) {
	return resolveParty(ctx, s.policies, policy.ResolveRequest{
		WalletID:        side.WalletID,
		WalletType:      side.WalletType,
		Currency:        request.Currency,
		Tier:            side.Tier,
		TransactionType: request.TransactionType,
		Channel:         request.Channel,
		Direction:       direction,
	}, name)
}

// resolveParty returns the policies covering one side (name) of a
// transaction; a side without any policy is a bad request.
func resolveParty(ctx context.Context, resolver PolicyResolver, request policy.ResolveRequest, name string) ([]policy.Policy, error) {
	policies, err := resolver.Resolve(ctx, request)
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
		if sender.Violated() || receiver.Violated() {
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
