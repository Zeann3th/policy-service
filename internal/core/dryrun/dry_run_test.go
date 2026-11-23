package dryrun

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/reservation"
)

func d(value string) decimal.Decimal { return decimal.RequireFromString(value) }

type resolverStub map[policy.Direction][]policy.Policy

func (r resolverStub) Resolve(_ context.Context, request policy.ResolveRequest) ([]policy.Policy, error) {
	return r[request.Direction], nil
}

// usageStub returns the same live total for every window and counts reads.
type usageStub struct {
	live  decimal.Decimal
	reads int
}

func (u *usageStub) Usage(context.Context, reservation.UsageKey) (decimal.Decimal, error) {
	u.reads++
	return u.live, nil
}

func dailyLimit(id int64, direction policy.Direction, limit string) policy.Policy {
	return policy.Policy{ID: id, PolicyTypeID: 1, PolicyType: "TOTAL_MAX", Kind: policy.KindLimit, Unit: policy.UnitAmount, TransactionType: policy.All, Channel: policy.All, Direction: direction, WindowType: policy.WindowDaily, Operator: policy.OpLTE, Value: d(limit)}
}

func fee(id int64, value string) policy.Policy {
	return policy.Policy{ID: id, PolicyTypeID: 2, PolicyType: "TXN_FEE", Kind: policy.KindFee, TransactionType: policy.All, Channel: policy.All, Direction: policy.DirectionOut, WindowType: policy.WindowNone, Base: policy.BaseAmount, Value: d(value)}
}

func request(amount string) Request {
	value := d(amount)
	return Request{
		TransactionType: "PAYMENT", Currency: "VND", Amount: &value,
		Sender:   reservation.PartyRequest{WalletID: "s", WalletType: "PERSONAL", Tier: "T1"},
		Receiver: reservation.PartyRequest{WalletID: "r", WalletType: "PERSONAL", Tier: "T1"},
	}
}

func policies() resolverStub {
	return resolverStub{
		policy.DirectionOut: {dailyLimit(1, policy.DirectionOut, "1000"), fee(2, "1")},
		policy.DirectionIn:  {dailyLimit(3, policy.DirectionIn, "5000")},
	}
}

func TestRunEvaluatesAgainstLiveUsage(t *testing.T) {
	usage := &usageStub{live: d("700")}
	response, err := NewService(policies(), usage).Run(context.Background(), request("400"))
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != reservation.StatusRejected || !response.Sender.IsViolated || response.Receiver.IsViolated {
		t.Fatalf("700 + 400 must break only the sender's 1000 limit: %#v", response)
	}
	if !response.Sender.Fee.Equal(d("4")) {
		t.Fatalf("fee = %s, want 1%% of 400", response.Sender.Fee)
	}
	// only the tracked policies read usage: one per side
	if usage.reads != 2 || len(response.Sender.Usage) != 1 || !response.Sender.Usage[0].Live.Equal(d("700")) || response.Sender.Usage[0].Overridden {
		t.Fatalf("reads = %d, sender usage = %#v", usage.reads, response.Sender.Usage)
	}
}

func TestRunAppliesUsageOverrides(t *testing.T) {
	value := request("400")
	value.UsageOverrides.Sender = map[int64]decimal.Decimal{1: d("0")}
	response, err := NewService(policies(), &usageStub{live: d("700")}).Run(context.Background(), value)
	if err != nil {
		t.Fatal(err)
	}
	line := response.Sender.Usage[0]
	if response.Status != reservation.StatusReserved || !line.Overridden || !line.Live.Equal(d("700")) || !line.Used.Equal(d("0")) {
		t.Fatalf("an override of 0 must let 400 through and keep the live value visible: %#v %#v", response.Status, line)
	}
}

func TestRunRejectsInvalidTransaction(t *testing.T) {
	value := request("400")
	value.Receiver.WalletID = value.Sender.WalletID
	if _, err := NewService(policies(), &usageStub{}).Run(context.Background(), value); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("same wallet: err = %v, want bad request", err)
	}
	if _, err := NewService(resolverStub{}, &usageStub{}).Run(context.Background(), request("1")); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("no policy: err = %v, want bad request", err)
	}
}
