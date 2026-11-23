package reservation

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

var at = time.Date(2026, time.November, 30, 18, 30, 0, 0, time.UTC)

func d(value string) decimal.Decimal { return decimal.RequireFromString(value) }

func p(id int64, kind policy.Kind, unitCode string, window policy.WindowType, operator policy.Operator, value string) policy.Policy {
	return policy.Policy{ID: id, PolicyTypeID: id, PolicyType: "TYPE", Kind: kind, Unit: unitCode, TransactionType: policy.All, Channel: policy.All, Direction: policy.DirectionOut, WindowType: window, Operator: operator, Value: d(value)}
}

func TestWindowStartUsesUTCCalendar(t *testing.T) {
	for window, want := range map[policy.WindowType]string{
		policy.WindowDaily:    "2026-11-30",
		policy.WindowMonthly:  "2026-11-01",
		policy.WindowLifetime: "1970-01-01",
	} {
		// 01:30 on Dec 1 in UTC+7 still counts toward Nov 30 in UTC
		if got := windowStart(window, at).Format(time.DateOnly); got != want {
			t.Errorf("windowStart(%s) = %s, want %s", window, got, want)
		}
	}
}

func TestEvaluateLimits(t *testing.T) {
	daily := p(1, policy.KindLimit, unit.CodeAmount, policy.WindowDaily, policy.OpLTE, "1000")
	single := p(2, policy.KindLimit, unit.CodeAmount, policy.WindowNone, policy.OpLTE, "500")
	balance := p(3, policy.KindLimit, unit.CodeBalance, policy.WindowNone, policy.OpGTE, "0")
	usage := map[UsageKey]decimal.Decimal{usageKey("w", daily, at): d("700")}

	result, entries, err := evaluate("w", d("400"), []policy.Policy{daily, single, balance}, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AppliedPolicies) != 2 {
		t.Fatalf("the balance limit must be skipped: %#v", result.AppliedPolicies)
	}
	if got := result.AppliedPolicies[0].Usage; got == nil || !got.Equal(d("1100")) {
		t.Fatalf("projected usage = %v", got)
	}
	if !result.IsViolated || !result.AppliedPolicies[0].Violated || result.AppliedPolicies[1].Violated {
		t.Fatal("700 + 400 must violate only the daily 1000 limit")
	}
	if len(entries) != 1 || !entries[0].Delta.Equal(d("400")) || entries[0].EntryType != EntryReserve || *entries[0].PolicyID != 1 {
		t.Fatalf("entries = %#v", entries)
	}

	result, _, err = evaluate("w", d("600"), []policy.Policy{single}, nil, at)
	if err != nil || !result.IsViolated {
		t.Fatalf("600 must violate the single 500 limit: %#v, %v", result, err)
	}
}

func TestEvaluateFeesWaiverDiscountTax(t *testing.T) {
	fee := p(1, policy.KindFee, unit.CodeAmount, policy.WindowNone, policy.OpEQ, "1000")
	percentFee := p(2, policy.KindFee, unit.CodePercent, policy.WindowNone, policy.OpEQ, "1")
	discount := p(3, policy.KindDiscount, unit.CodePercent, policy.WindowNone, policy.OpEQ, "10")
	tax := p(4, policy.KindTax, unit.CodePercent, policy.WindowNone, policy.OpEQ, "10")
	waiver := p(5, policy.KindWaiver, unit.CodeCount, policy.WindowMonthly, policy.OpLT, "3")
	all := []policy.Policy{fee, percentFee, discount, tax, waiver}

	// 3rd transaction this month: the waiver is used up
	usage := map[UsageKey]decimal.Decimal{usageKey("w", waiver, at): d("3")}
	result, entries, err := evaluate("w", d("100000"), all, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	// fee 1000 + 1% of 100000 = 2000; discount 10% = 200; tax 10% of 1800 = 180
	if !result.Fee.Equal(d("2000")) || !result.Discount.Equal(d("200")) || !result.Tax.Equal(d("180")) {
		t.Fatalf("fee/discount/tax = %s/%s/%s", result.Fee, result.Discount, result.Tax)
	}
	if len(entries) != 1 || !entries[0].Delta.Equal(d("1")) {
		t.Fatalf("waiver must count one transaction: %#v", entries)
	}

	// 1st transaction this month: free
	result, _, err = evaluate("w", d("100000"), all, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Fee.IsZero() || !result.Discount.IsZero() || !result.Tax.IsZero() {
		t.Fatalf("waived fee/discount/tax = %s/%s/%s", result.Fee, result.Discount, result.Tax)
	}
}

func TestEvaluateRejectsUnsupportedUnits(t *testing.T) {
	for _, value := range []policy.Policy{
		p(1, policy.KindFee, unit.CodeCount, policy.WindowNone, policy.OpEQ, "1"),
		p(2, policy.KindLimit, unit.CodePercent, policy.WindowDaily, policy.OpLTE, "1"),
	} {
		if _, _, err := evaluate("w", d("1"), []policy.Policy{value}, nil, at); err == nil {
			t.Errorf("policy %d must fail", value.ID)
		}
	}
}
