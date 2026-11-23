package policy

import (
	"testing"
	"time"

	cachecore "github.com/zeann3th/policy-service/internal/core/cache"

	"github.com/shopspring/decimal"
)

func TestMergePoliciesOverridesOnlyExactSelector(t *testing.T) {
	system := []Policy{
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionAll, Value: decimal.NewFromInt(100)},
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionOut, WindowType: WindowDaily, Value: decimal.NewFromInt(200)},
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionOut, WindowType: WindowMonthly, Value: decimal.NewFromInt(300)},
	}
	custom := []CustomPolicy{
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionOut, WindowType: WindowDaily, Value: decimal.NewFromInt(50)},
		{PolicyTypeID: 1, TransactionType: "TRANSFER", Channel: All, Direction: DirectionOut, WindowType: WindowNone, Value: decimal.NewFromInt(25)},
	}

	got := mergePolicies(system, custom)

	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	if !got[0].Value.Equal(decimal.NewFromInt(100)) || got[0].Source != SourceSystem {
		t.Fatalf("ALL direction changed: %#v", got[0])
	}
	if !got[1].Value.Equal(decimal.NewFromInt(50)) || got[1].Source != SourceCustom || got[1].WindowType != WindowDaily {
		t.Fatalf("OUT direction not overridden: %#v", got[1])
	}
	if !got[2].Value.Equal(decimal.NewFromInt(300)) || got[2].WindowType != WindowMonthly || got[2].Source != SourceSystem {
		t.Fatalf("monthly policy changed: %#v", got[2])
	}
	if got[3].TransactionType != "TRANSFER" || got[3].Source != SourceCustom || got[3].WindowType != WindowNone {
		t.Fatalf("specific selector not appended: %#v", got[3])
	}
}

func TestFilterPoliciesKeepsGlobalAndMatchingTransaction(t *testing.T) {
	values := []Policy{{TransactionType: All}, {TransactionType: "TRANSFER"}, {TransactionType: "PAYMENT"}}
	got := filterPoliciesByTransactionType(values, "TRANSFER")
	if len(got) != 2 || got[0].TransactionType != All || got[1].TransactionType != "TRANSFER" {
		t.Fatalf("filtered = %#v", got)
	}
}

func TestPolicyResponseIncludesWindowType(t *testing.T) {
	for _, test := range []struct {
		input WindowType
		want  WindowType
	}{
		{input: WindowDaily, want: WindowDaily},
		{input: "", want: WindowNone},
	} {
		response := policyResponse(Policy{WindowType: test.input})
		if response.WindowType != test.want {
			t.Fatalf("policyResponse(%q).windowType = %q, want %q", test.input, response.WindowType, test.want)
		}
	}
}

func TestCacheTTLStopsAtNextEffectiveBoundary(t *testing.T) {
	now := time.Date(2026, time.November, 23, 10, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { value := now.Add(d); return &value }
	for _, test := range []struct {
		name      string
		policies  []Policy
		nextStart *time.Time
		want      time.Duration
	}{
		{name: "no boundary", policies: []Policy{{}}, want: cachecore.TTL},
		{name: "boundary after TTL", nextStart: at(time.Hour), want: cachecore.TTL},
		{name: "next start", nextStart: at(5 * time.Minute), want: 5 * time.Minute},
		{name: "earliest end", policies: []Policy{{EffectiveTo: at(3 * time.Minute)}, {EffectiveTo: at(7 * time.Minute)}}, nextStart: at(5 * time.Minute), want: 3 * time.Minute},
	} {
		if got := cacheTTL(test.policies, test.nextStart, now); got != test.want {
			t.Errorf("%s: ttl = %s, want %s", test.name, got, test.want)
		}
	}
}

func TestKindCheckEnforcesShape(t *testing.T) {
	for _, test := range []struct {
		kind     Kind
		unit     Unit
		base     Base
		operator Operator
		window   WindowType
		ok       bool
	}{
		{KindLimit, UnitAmount, "", OpLTE, WindowDaily, true},
		{KindLimit, UnitCount, "", OpLTE, WindowNone, true},
		{KindLimit, "", "", OpLTE, WindowNone, false},
		{KindLimit, UnitAmount, "", "", WindowNone, false},
		{KindLimit, UnitAmount, BaseAmount, OpLTE, WindowNone, false},
		{KindWaiver, UnitCount, "", "", WindowMonthly, true},
		{KindWaiver, UnitAmount, "", "", WindowMonthly, false},
		{KindWaiver, UnitCount, "", OpLT, WindowMonthly, false},
		{KindFee, "", "", "", WindowNone, true},
		{KindFee, "", BaseAmount, "", WindowNone, true},
		{KindFee, "", BaseFee, "", WindowNone, false},
		{KindFee, UnitAmount, "", "", WindowNone, false},
		{KindFee, "", "", OpEQ, WindowNone, false},
		{KindFee, "", "", "", WindowDaily, false},
		{KindDiscount, "", BaseFee, "", WindowNone, true},
		{KindDiscount, "", BaseNetFee, "", WindowNone, false},
		{KindTax, "", BaseNetFee, "", WindowNone, true},
		{"REVENUE_SHARE", "", "", "", WindowNone, false},
	} {
		if err := test.kind.Check(test.unit, test.base, test.operator, test.window); (err == nil) != test.ok {
			t.Errorf("%s unit=%q base=%q operator=%q window=%s: err = %v, want ok %v", test.kind, test.unit, test.base, test.operator, test.window, err, test.ok)
		}
	}
}
