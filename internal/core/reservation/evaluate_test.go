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

// unit ids as seeded in the units table
var unitIDs = map[string]int64{policy.UnitAmount: 1, policy.UnitCount: 2, "BALANCE": 3}

// p is a LIMIT or WAIVER policy measured in unit.
func p(id int64, kind policy.Kind, unit string, window policy.WindowType, operator policy.Operator, value string) policy.Policy {
	value_ := policy.Policy{ID: id, PolicyTypeID: id, PolicyType: "TYPE", Kind: kind, TransactionType: policy.All, Channel: policy.All, Direction: policy.DirectionOut, WindowType: window, Operator: operator, Value: d(value), IsEnforced: true}
	if unit != "" {
		unitID := unitIDs[unit]
		value_.UnitID, value_.Unit = &unitID, unit
	}
	return value_
}

// the charge kinds, in the order they are computed
const (
	TypeFee      = policy.KindFee
	TypeDiscount = policy.KindDiscount
	TypeTax      = policy.KindTax
)

var chargeKinds = []policy.Kind{TypeFee, TypeDiscount, TypeTax}

// c is a charge of kind: flat without a base, otherwise a percentage of it.
func c(id int64, kind policy.Kind, base policy.Base, value string) policy.Policy {
	charge := p(id, kind, "", policy.WindowTransaction, "", value)
	charge.PolicyType, charge.Base = "TXN_"+string(kind), base
	return charge
}

// windowCapped gives a charge a window cap counted in unit.
func windowCapped(value policy.Policy, window policy.WindowType, unit, cap string) policy.Policy {
	unitID, amount := unitIDs[unit], d(cap)
	value.WindowType, value.UnitID, value.Unit, value.Cap = window, &unitID, unit, &amount
	return value
}

func charges(result PartyResult) map[policy.Kind]string {
	totals := map[policy.Kind]string{}
	for _, charge := range result.Charges {
		totals[charge.Kind] = charge.Amount.String()
	}
	return totals
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

func TestEveryKindHasOneStrategyInOrder(t *testing.T) {
	var kinds []policy.Kind
	for _, value := range strategies.order {
		kinds = append(kinds, value.kind())
	}
	if want := policy.Kinds(); !slices.Equal(kinds, want) {
		t.Fatalf("strategies = %v, want %v (limits and the waiver before the charges)", kinds, want)
	}
}

func TestRegistryRejectsDuplicateKind(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("registering a kind twice must panic")
		}
	}()
	newRegistry(limitStrategy{}, limitStrategy{})
}

func TestEngineSkipsConfigOnlyAndUnknownPolicies(t *testing.T) {
	// config only: the type is not enforced (balance limits for accounting)
	balance := p(1, policy.KindLimit, "BALANCE", policy.WindowTransaction, policy.OpGTE, "1000000")
	balance.IsEnforced = false
	// enforced, but the code has no behaviour for them: they do nothing
	unmeasured := p(2, policy.KindLimit, "BALANCE", policy.WindowTransaction, policy.OpGTE, "1000000")
	smsFee := c(3, "COMMISSION", "", "500")
	unknownKind := c(4, "REVENUE_SHARE", "", "1")
	unknownKind.Kind = "REVENUE_SHARE"
	fee := c(5, TypeFee, "", "1100")

	result, entries, err := evaluate("w", d("1"), []policy.Policy{balance, unmeasured, smsFee, unknownKind, fee}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.AppliedPolicies) != 1 || result.AppliedPolicies[0].PolicyID != 5 || len(entries) != 0 || result.Violated() {
		t.Fatalf("only the fee may reach the engine: %#v", result.AppliedPolicies)
	}
	if want := map[policy.Kind]string{TypeFee: "1100"}; !equalMaps(charges(result), want) {
		t.Fatalf("charges = %v, want %v", charges(result), want)
	}
}

func TestEvaluateLimits(t *testing.T) {
	daily := p(1, policy.KindLimit, policy.UnitAmount, policy.WindowDaily, policy.OpLTE, "1000")
	single := p(2, policy.KindLimit, policy.UnitAmount, policy.WindowTransaction, policy.OpLTE, "500")
	usage := map[UsageKey]decimal.Decimal{usageKey("w", daily, at): d("700")}

	result, entries, err := evaluate("w", d("400"), []policy.Policy{daily, single}, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.AppliedPolicies[0].Usage; got == nil || !got.Equal(d("1100")) {
		t.Fatalf("projected usage = %v", got)
	}
	if !result.Violated() || !result.AppliedPolicies[0].Violated || result.AppliedPolicies[1].Violated {
		t.Fatal("700 + 400 must violate only the daily 1000 limit")
	}
	if len(entries) != 1 || !entries[0].Delta.Equal(d("400")) || entries[0].EntryType != EntryReserve || *entries[0].PolicyID != 1 || entries[0].UnitID != unitIDs[policy.UnitAmount] {
		t.Fatalf("entries = %#v", entries)
	}

	result, _, err = evaluate("w", d("600"), []policy.Policy{single}, nil, at)
	if err != nil || !result.Violated() {
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

func TestEvaluateWaiverFeeDiscountTax(t *testing.T) {
	fee := c(1, TypeFee, "", "1000")
	percentFee := c(2, TypeFee, policy.BaseAmount, "1")
	discount := c(3, TypeDiscount, policy.BaseFee, "10")
	tax := c(4, TypeTax, policy.BaseNetFee, "10")
	waiver := p(5, policy.KindWaiver, policy.UnitCount, policy.WindowMonthly, "", "3")
	// any input order: the waiver runs before the charges, the fee before the tax
	all := []policy.Policy{tax, discount, fee, percentFee, waiver}

	// 4th transaction this month: the waiver is used up
	usage := map[UsageKey]decimal.Decimal{usageKey("w", waiver, at): d("3")}
	result, entries, err := evaluate("w", d("100000"), all, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	// fee 1000 + 1% of 100000 = 2000; discount 10% = 200 off; tax 10% of 1800 = 180
	want := map[policy.Kind]string{TypeFee: "2000", TypeDiscount: "200", TypeTax: "180"}
	if got := charges(result); !equalMaps(got, want) {
		t.Fatalf("charges = %v, want %v", got, want)
	}
	if order := []policy.Kind{result.Charges[0].Kind, result.Charges[1].Kind, result.Charges[2].Kind}; !slices.Equal(order, chargeKinds) {
		t.Fatalf("charges order = %v", order)
	}
	if len(entries) != 1 || !entries[0].Delta.Equal(d("1")) || entries[0].Operator != nil {
		t.Fatalf("waiver must count one transaction: %#v", entries)
	}

	// 1st transaction this month: the fee is waived, so is the tax on it
	result, _, err = evaluate("w", d("100000"), all, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[policy.Kind]string{TypeFee: "0", TypeDiscount: "0", TypeTax: "0"}; !equalMaps(charges(result), want) {
		t.Fatalf("waived charges = %v", charges(result))
	}
}

func TestEvaluateChargesUseTheirBase(t *testing.T) {
	fee := c(1, TypeFee, policy.BaseAmount, "1")
	feeDiscount := c(2, TypeDiscount, policy.BaseFee, "50")
	cashback := c(3, TypeDiscount, policy.BaseAmount, "2")
	for name, test := range map[string]struct {
		tax  policy.Policy
		want string
	}{
		// fee 1% of 100000 = 1000; fee discount 50% = 500; cashback 2% of 100000 = 2000
		"tax on net fee": {tax: c(4, TypeTax, policy.BaseNetFee, "10"), want: "50"},
		"tax on fee":     {tax: c(4, TypeTax, policy.BaseFee, "10"), want: "100"},
		"tax on amount":  {tax: c(4, TypeTax, policy.BaseAmount, "10"), want: "10000"},
		"flat tax":       {tax: c(4, TypeTax, "", "10"), want: "10"},
	} {
		result, _, err := evaluate("w", d("100000"), []policy.Policy{fee, feeDiscount, cashback, test.tax}, nil, at)
		if err != nil {
			t.Fatal(err)
		}
		if want := map[policy.Kind]string{TypeFee: "1000", TypeDiscount: "2500", TypeTax: test.want}; !equalMaps(charges(result), want) {
			t.Errorf("%s: charges = %v, want %v", name, charges(result), want)
		}
	}
}

func TestEvaluateCapsDiscountsByTheirBase(t *testing.T) {
	fee := c(1, TypeFee, "", "100")
	feeDiscount := c(2, TypeDiscount, policy.BaseFee, "300")
	flatDiscount := c(3, TypeDiscount, "", "50")
	cashback := c(4, TypeDiscount, policy.BaseAmount, "150")
	result, _, err := evaluate("w", d("1000"), []policy.Policy{fee, feeDiscount, flatDiscount, cashback}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	// flat and fee discounts at most the fee (100), cashback at most the amount (1000)
	if got := charges(result)[TypeDiscount]; got != "1100" {
		t.Fatalf("discount = %s, want -1100", got)
	}
	// each line reports its own signed charge
	if got := result.AppliedPolicies[1].Charge; got == nil || got.String() != "300" {
		t.Fatalf("fee discount line = %v, want -300", got)
	}
}

func TestEvaluateCapsChargesPerWindow(t *testing.T) {
	// 0.5% fee, at most 1,000 charged per day; 800 charged already today
	fee := windowCapped(c(1, TypeFee, policy.BaseAmount, "0.5"), policy.WindowDaily, policy.UnitAmount, "1000")
	usage := map[UsageKey]decimal.Decimal{usageKey("w", fee, at): d("800")}
	result, entries, err := evaluate("w", d("100000"), []policy.Policy{fee}, usage, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := charges(result)[TypeFee]; got != "200" {
		t.Fatalf("fee = %s, want the 200 left of the daily cap", got)
	}
	if len(entries) != 1 || !entries[0].Delta.Equal(d("200")) || !entries[0].LimitValue.Equal(d("1000")) {
		t.Fatalf("entries = %#v", entries)
	}

	// cashback on the first 2 payments a month only (cap counted in COUNT)
	cashback := windowCapped(c(2, TypeDiscount, policy.BaseAmount, "1"), policy.WindowMonthly, policy.UnitCount, "2")
	for used, want := range map[string]string{"1": "1000", "2": "0"} {
		usage := map[UsageKey]decimal.Decimal{usageKey("w", cashback, at): d(used)}
		result, entries, err := evaluate("w", d("100000"), []policy.Policy{cashback}, usage, at)
		if err != nil {
			t.Fatal(err)
		}
		if got := charges(result)[TypeDiscount]; got != want {
			t.Errorf("after %s cashbacks: discount = %s, want %s", used, got, want)
		}
		if wantEntries := map[string]int{"1": 1, "2": 0}[used]; len(entries) != wantEntries {
			t.Errorf("after %s cashbacks: %d entries, want %d", used, len(entries), wantEntries)
		}
	}
}

func TestEvaluateCapsChargesPerTransaction(t *testing.T) {
	// 1% fee, at most 50,000 per transaction; 10% off the fee, at most 3,000
	fee := c(1, TypeFee, policy.BaseAmount, "1")
	feeCap, discountCap := d("50000"), d("3000")
	fee.Cap = &feeCap
	discount := c(2, TypeDiscount, policy.BaseFee, "10")
	discount.Cap = &discountCap
	for amount, want := range map[string]map[policy.Kind]string{
		"1000000":  {TypeFee: "10000", TypeDiscount: "1000"}, // under both caps
		"10000000": {TypeFee: "50000", TypeDiscount: "3000"}, // 100,000 fee capped; 5,000 discount capped
	} {
		result, entries, err := evaluate("w", d(amount), []policy.Policy{fee, discount}, nil, at)
		if err != nil {
			t.Fatal(err)
		}
		if got := charges(result); !equalMaps(got, want) {
			t.Errorf("amount %s: charges = %v, want %v", amount, got, want)
		}
		if len(entries) != 0 {
			t.Errorf("amount %s: a per-transaction cap keeps no usage, got %#v", amount, entries)
		}
		if result.AppliedPolicies[0].Usage != nil {
			t.Errorf("amount %s: usage = %v, want none", amount, result.AppliedPolicies[0].Usage)
		}
	}
}

func TestEvaluateTransactionWaiverAlwaysWaives(t *testing.T) {
	waiver := p(1, policy.KindWaiver, policy.UnitCount, policy.WindowTransaction, "", "1")
	result, entries, err := evaluate("w", d("100000"), []policy.Policy{waiver, c(2, TypeFee, "", "1100")}, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	if got := charges(result)[TypeFee]; got != "0" || len(entries) != 0 {
		t.Fatalf("fee = %s, entries = %#v; want waived and nothing recorded", got, entries)
	}
}

func TestChargesAreNeverNegative(t *testing.T) {
	all := []policy.Policy{c(1, TypeFee, "", "1000"), c(2, TypeDiscount, policy.BaseFee, "10"), c(3, TypeDiscount, policy.BaseAmount, "1"), c(4, TypeTax, policy.BaseNetFee, "10")}
	result, _, err := evaluate("w", d("100000"), all, nil, at)
	if err != nil {
		t.Fatal(err)
	}
	for _, charge := range result.Charges {
		if charge.Amount.IsNegative() {
			t.Errorf("%s total = %s, want >= 0", charge.Kind, charge.Amount)
		}
	}
	for _, applied := range result.AppliedPolicies {
		if applied.Charge != nil && applied.Charge.IsNegative() {
			t.Errorf("policy %d line = %s, want >= 0", applied.PolicyID, applied.Charge)
		}
	}
}

func TestEachAppliedPolicySaysWhetherItIsViolated(t *testing.T) {
	min := p(1, policy.KindLimit, policy.UnitAmount, policy.WindowTransaction, policy.OpGTE, "10000")
	max := p(2, policy.KindLimit, policy.UnitAmount, policy.WindowTransaction, policy.OpLTE, "50000000")
	for amount, want := range map[string][2]bool{"100": {true, false}, "20000": {false, false}} {
		result, _, err := evaluate("w", d(amount), []policy.Policy{min, max}, nil, at)
		if err != nil {
			t.Fatal(err)
		}
		got := [2]bool{result.AppliedPolicies[0].Violated, result.AppliedPolicies[1].Violated}
		if got != want || result.Violated() != (want[0] || want[1]) {
			t.Errorf("amount %s: violated = %v (side %v), want %v", amount, got, result.Violated(), want)
		}
	}
}

func TestEvaluateRejectsBaseNotYetComputed(t *testing.T) {
	for name, value := range map[string]policy.Policy{
		"fee on fee":       c(1, TypeFee, policy.BaseFee, "1"),
		"discount net fee": c(1, TypeDiscount, policy.BaseNetFee, "1"),
	} {
		if _, _, err := evaluate("w", d("100"), []policy.Policy{value}, nil, at); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestRulesCheckWhatEachKindTakes(t *testing.T) {
	cap := d("50000")
	negative := d("-10")
	spec := func(kind policy.Kind, unit string, base policy.Base, operator policy.Operator, window policy.WindowType) policy.Spec {
		return policy.Spec{Kind: kind, Unit: unit, Base: base, Operator: operator, Window: window, Value: d("1")}
	}
	withCap := func(value policy.Spec, amount *decimal.Decimal) policy.Spec { value.Cap = amount; return value }
	withValue := func(value policy.Spec, amount decimal.Decimal) policy.Spec { value.Value = amount; return value }
	for name, test := range map[string]struct {
		spec policy.Spec
		ok   bool
	}{
		"daily amount limit":     {spec(policy.KindLimit, policy.UnitAmount, "", policy.OpLTE, policy.WindowDaily), true},
		"balance limit":          {spec(policy.KindLimit, "BALANCE", "", policy.OpGTE, policy.WindowTransaction), true},
		"limit without unit":     {spec(policy.KindLimit, "", "", policy.OpLTE, policy.WindowTransaction), false},
		"limit without operator": {spec(policy.KindLimit, policy.UnitAmount, "", "", policy.WindowTransaction), false},
		"limit with base":        {spec(policy.KindLimit, policy.UnitAmount, policy.BaseAmount, policy.OpLTE, policy.WindowTransaction), false},
		"monthly count waiver":   {spec(policy.KindWaiver, policy.UnitCount, "", "", policy.WindowMonthly), true},
		"waiver with operator":   {spec(policy.KindWaiver, policy.UnitCount, "", policy.OpLT, policy.WindowMonthly), false},
		"flat fee":               {spec(TypeFee, "", "", "", policy.WindowTransaction), true},
		"fee on the amount":      {spec(TypeFee, "", policy.BaseAmount, "", policy.WindowTransaction), true},
		"fee on the fee":         {spec(TypeFee, "", policy.BaseFee, "", policy.WindowTransaction), false},
		"tax on net fee":         {spec(TypeTax, "", policy.BaseNetFee, "", policy.WindowTransaction), true},
		"charge with operator":   {spec(TypeFee, "", "", policy.OpEQ, policy.WindowTransaction), false},
		"daily capped fee":       {withCap(spec(TypeFee, policy.UnitAmount, policy.BaseAmount, "", policy.WindowDaily), &cap), true},
		"window without cap":     {spec(TypeFee, policy.UnitAmount, "", "", policy.WindowDaily), false},
		"per-transaction cap":    {withCap(spec(TypeFee, "", policy.BaseAmount, "", policy.WindowTransaction), &cap), true},
		"per-transaction unit":   {withCap(spec(TypeFee, policy.UnitAmount, policy.BaseAmount, "", policy.WindowTransaction), &cap), false},
		"window cap on balance":  {withCap(spec(TypeFee, "BALANCE", "", "", policy.WindowDaily), &cap), false},
		"window without unit":    {withCap(spec(TypeFee, "", "", "", policy.WindowDaily), &cap), false},
		"waiver with cap":        {withCap(spec(policy.KindWaiver, policy.UnitCount, "", "", policy.WindowMonthly), &cap), false},
		"limit with cap":         {withCap(spec(policy.KindLimit, policy.UnitAmount, "", policy.OpLTE, policy.WindowDaily), &cap), false},
		"unknown operator":       {spec(policy.KindLimit, policy.UnitAmount, "", "ABOUT", policy.WindowDaily), false},
		"negative discount":      {withValue(spec(TypeDiscount, "", policy.BaseAmount, "", policy.WindowTransaction), negative), false},
		"negative cap":           {withCap(spec(TypeFee, policy.UnitAmount, "", "", policy.WindowDaily), &negative), false},
		"discount on the fee":    {spec(TypeDiscount, "", policy.BaseFee, "", policy.WindowTransaction), true},
		"discount on net fee":    {spec(TypeDiscount, "", policy.BaseNetFee, "", policy.WindowTransaction), false},
		"auxiliary is gone":      {spec("AUXILIARY", "", "", "", policy.WindowTransaction), false},
		"unknown kind":           {spec("REVENUE_SHARE", "", "", "", policy.WindowTransaction), false},
		"unknown window":         {spec(policy.KindLimit, policy.UnitAmount, "", policy.OpLTE, "WEEKLY"), false},
	} {
		if err := NewRules().Check(test.spec); (err == nil) != test.ok {
			t.Errorf("%s: err = %v, want ok %v", name, err, test.ok)
		}
	}
}

func equalMaps(got, want map[policy.Kind]string) bool {
	if len(got) != len(want) {
		return false
	}
	for key, value := range want {
		if got[key] != value {
			return false
		}
	}
	return true
}
