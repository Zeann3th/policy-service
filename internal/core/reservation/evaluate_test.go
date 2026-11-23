package reservation

import (
	"slices"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

var at = time.Date(2026, time.November, 30, 18, 30, 0, 0, time.UTC)

func d(value string) decimal.Decimal { return decimal.RequireFromString(value) }

func p(id int64, kind policy.Kind, unit policy.Unit, window policy.WindowType, operator policy.Operator, value string) policy.Policy {
	return policy.Policy{ID: id, PolicyTypeID: id, PolicyType: "TYPE", Kind: kind, Unit: unit, TransactionType: policy.All, Channel: policy.All, Direction: policy.DirectionOut, WindowType: window, Operator: operator, Value: d(value)}
}

// c is a charge (FEE, DISCOUNT, TAX): flat without a base, otherwise a
// percentage of it.
func c(id int64, kind policy.Kind, base policy.Base, value string) policy.Policy {
	charge := p(id, kind, "", policy.WindowNone, "", value)
	charge.Base = base
	return charge
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

func TestEveryKindHasOneStrategy(t *testing.T) {
	var kinds []policy.Kind
	for _, value := range strategies.order {
		kinds = append(kinds, value.kind())
	}
	if want := policy.Kinds(); !slices.Equal(slices.Sorted(slices.Values(kinds)), want) {
		t.Fatalf("strategies cover %v, declared kinds are %v", kinds, want)
	}
	// charges read the bases of earlier strategies; the waiver zeroes the fee
	want := []policy.Kind{policy.KindLimit, policy.KindWaiver, policy.KindFee, policy.KindDiscount, policy.KindTax}
	if !slices.Equal(kinds, want) {
		t.Fatalf("order = %v, want %v", kinds, want)
	}
}

func TestRegistryRejectsDuplicateKind(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("registering a kind twice must panic")
		}
	}()
	newRegistry(feeStrategy{}, feeStrategy{})
}

func TestEvaluateRejectsKindWithoutStrategy(t *testing.T) {
	if _, _, err := evaluate("w", d("1"), []policy.Policy{c(1, "REVENUE_SHARE", "", "1")}, nil, at); err == nil {
		t.Fatal("want error")
	}
}

func TestEvaluateLimits(t *testing.T) {
	daily := p(1, policy.KindLimit, policy.UnitAmount, policy.WindowDaily, policy.OpLTE, "1000")
	single := p(2, policy.KindLimit, policy.UnitAmount, policy.WindowNone, policy.OpLTE, "500")
	balance := p(3, policy.KindLimit, policy.UnitBalance, policy.WindowNone, policy.OpGTE, "0")
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
	if len(entries) != 1 || !entries[0].Delta.Equal(d("400")) || entries[0].EntryType != EntryReserve || *entries[0].PolicyID != 1 || entries[0].Unit != policy.UnitAmount {
		t.Fatalf("entries = %#v", entries)
	}

	result, _, err = evaluate("w", d("600"), []policy.Policy{single}, nil, at)
	if err != nil || !result.IsViolated {
		t.Fatalf("600 must violate the single 500 limit: %#v, %v", result, err)
	}
}

func TestEvaluateCountLimitCountsTransactions(t *testing.T) {
	count := p(1, policy.KindLimit, policy.UnitCount, policy.WindowDaily, policy.OpLTE, "2")
	amount := p(1, policy.KindLimit, policy.UnitAmount, policy.WindowDaily, policy.OpLTE, "1000000")
	usage := map[UsageKey]decimal.Decimal{usageKey("w", count, at): d("2"), usageKey("w", amount, at): d("10")}
	result, entries, err := evaluate("w", d("500"), []policy.Policy{count, amount}, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AppliedPolicies[0].Violated || result.AppliedPolicies[1].Violated {
		t.Fatalf("the 3rd transaction must violate only the count limit: %#v", result.AppliedPolicies)
	}
	// same type and scope, different unit: separate usage rows
	if len(entries) != 2 || entries[0].UsageKey == entries[1].UsageKey || !entries[0].Delta.Equal(one) || !entries[1].Delta.Equal(d("500")) {
		t.Fatalf("entries = %#v", entries)
	}
}

func TestEvaluateFeesWaiverDiscountTax(t *testing.T) {
	fee := c(1, policy.KindFee, "", "1000")
	percentFee := c(2, policy.KindFee, policy.BaseAmount, "1")
	discount := c(3, policy.KindDiscount, policy.BaseFee, "10")
	tax := c(4, policy.KindTax, policy.BaseNetFee, "10")
	waiver := p(5, policy.KindWaiver, policy.UnitCount, policy.WindowMonthly, "", "3")
	// any input order: the registry processes the waiver before the fee
	all := []policy.Policy{tax, discount, fee, percentFee, waiver}

	// 4th transaction this month: the waiver is used up
	usage := map[UsageKey]decimal.Decimal{usageKey("w", waiver, at): d("3")}
	result, entries, err := evaluate("w", d("100000"), all, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	// fee 1000 + 1% of 100000 = 2000; discount 10% = 200; tax 10% of 1800 = 180
	if !result.Fee.Equal(d("2000")) || !result.Discount.Equal(d("200")) || !result.Tax.Equal(d("180")) {
		t.Fatalf("fee/discount/tax = %s/%s/%s", result.Fee, result.Discount, result.Tax)
	}
	if len(entries) != 1 || !entries[0].Delta.Equal(d("1")) || entries[0].Operator != nil {
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

func TestEvaluateChargesUseTheirBase(t *testing.T) {
	fee := c(1, policy.KindFee, policy.BaseAmount, "1")
	feeDiscount := c(2, policy.KindDiscount, policy.BaseFee, "50")
	cashback := c(3, policy.KindDiscount, policy.BaseAmount, "2")
	for name, test := range map[string]struct {
		tax               policy.Policy
		discount, wantTax string
	}{
		// fee 1% of 100000 = 1000; fee discount 50% = 500; cashback 2% of 100000 = 2000
		"tax on net fee": {tax: c(4, policy.KindTax, policy.BaseNetFee, "10"), discount: "2500", wantTax: "50"},
		"tax on fee":     {tax: c(4, policy.KindTax, policy.BaseFee, "10"), discount: "2500", wantTax: "100"},
		"tax on amount":  {tax: c(4, policy.KindTax, policy.BaseAmount, "10"), discount: "2500", wantTax: "10000"},
		"flat tax":       {tax: c(4, policy.KindTax, "", "10"), discount: "2500", wantTax: "10"},
	} {
		result, _, err := evaluate("w", d("100000"), []policy.Policy{fee, feeDiscount, cashback, test.tax}, nil, at)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Fee.Equal(d("1000")) || !result.Discount.Equal(d(test.discount)) || !result.Tax.Equal(d(test.wantTax)) {
			t.Errorf("%s: fee/discount/tax = %s/%s/%s", name, result.Fee, result.Discount, result.Tax)
		}
	}
}

func TestEvaluateCapsDiscountsByTheirBase(t *testing.T) {
	fee := c(1, policy.KindFee, "", "100")
	feeDiscount := c(2, policy.KindDiscount, policy.BaseFee, "300")
	flatDiscount := c(3, policy.KindDiscount, "", "50")
	cashback := c(4, policy.KindDiscount, policy.BaseAmount, "150")
	result, _, err := evaluate("w", d("1000"), []policy.Policy{fee, feeDiscount, flatDiscount, cashback}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	// flat and fee discounts capped at the fee (100), cashback at the amount (1000)
	if !result.Discount.Equal(d("1100")) {
		t.Fatalf("discount = %s, want 1100", result.Discount)
	}
}

func TestEvaluateRejectsBaseNotYetComputed(t *testing.T) {
	for name, value := range map[string]policy.Policy{
		"fee on fee":       c(1, policy.KindFee, policy.BaseFee, "1"),
		"discount net fee": c(1, policy.KindDiscount, policy.BaseNetFee, "1"),
	} {
		if _, _, err := evaluate("w", d("100"), []policy.Policy{value}, nil, at); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestEvaluateRejectsUnmeasurableLimit(t *testing.T) {
	for _, value := range []policy.Policy{
		p(1, policy.KindLimit, "", policy.WindowNone, policy.OpLTE, "1"),
		p(2, policy.KindWaiver, "", policy.WindowDaily, "", "1"),
	} {
		if _, _, err := evaluate("w", d("1"), []policy.Policy{value}, nil, at); err == nil {
			t.Errorf("policy %d must fail", value.ID)
		}
	}
}

func TestEvaluateReportsEachLineCharge(t *testing.T) {
	fee := c(1, policy.KindFee, policy.BaseAmount, "1")
	tax := c(2, policy.KindTax, policy.BaseFee, "10")
	waiver := p(3, policy.KindWaiver, policy.UnitCount, policy.WindowMonthly, "", "1")
	charges := func(result PartyResult) map[int64]string {
		lines := map[int64]string{}
		for _, applied := range result.AppliedPolicies {
			if applied.Charge != nil {
				lines[applied.PolicyID] = applied.Charge.String()
			}
		}
		return lines
	}

	// waiver used up: fee 1% of 100000 = 1000, tax 10% of it = 100
	used := map[UsageKey]decimal.Decimal{usageKey("w", waiver, at): d("1")}
	result, _, err := evaluate("w", d("100000"), []policy.Policy{fee, tax, waiver}, used, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := charges(result); got[1] != "1000" || got[2] != "100" || len(got) != 2 {
		t.Fatalf("line charges = %v", got)
	}

	// waived: the fee line charges nothing, so neither does the tax on it
	result, _, err = evaluate("w", d("100000"), []policy.Policy{fee, tax, waiver}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := charges(result); got[1] != "0" || got[2] != "0" {
		t.Fatalf("waived line charges = %v", got)
	}
}
