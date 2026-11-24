package reservation

import (
	"fmt"
	"slices"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// chargeStrategy is the template for the charge kinds. For every policy of
// its kind it prices the line on its base, lets adjust change it, applies
// the policy's cap (per transaction or per window) and records the usage;
// then settle turns the lines into the kind's total and publish makes it
// available as a base to later kinds. A nil hook keeps the default. Each
// charge kind is one value of it, registered in strategies in the order
// they are computed, so a kind may only use bases published before it.
type chargeStrategy struct {
	of    policy.Kind
	bases []policy.Base // what its policies may be a percentage of
	// adjust changes a priced line before the cap; default: unchanged.
	adjust func(e *evaluation, amount decimal.Decimal) decimal.Decimal
	// settle is the kind's total from its capped lines; default: their sum.
	settle func(e *evaluation, lines []chargeLine) decimal.Decimal
	// publish makes the total available as a base; default: nothing.
	publish func(e *evaluation, total decimal.Decimal)
}

type chargeLine struct {
	policy policy.Policy
	amount decimal.Decimal
}

var (
	// feeStrategy: paid; zero while a waiver applies; publishes base FEE.
	feeStrategy = chargeStrategy{
		of:    policy.KindFee,
		bases: []policy.Base{policy.BaseAmount},
		adjust: func(e *evaluation, amount decimal.Decimal) decimal.Decimal {
			if e.waived {
				return decimal.Zero
			}
			return amount
		},
		publish: func(e *evaluation, total decimal.Decimal) { e.bases[policy.BaseFee] = total },
	}
	// discountStrategy: taken off what the wallet pays. Flat and fee-based discounts come off
	// the fee (at most the fee) and publish NET_FEE; amount-based ones
	// (cashback) are at most the amount.
	discountStrategy = chargeStrategy{
		of:    policy.KindDiscount,
		bases: []policy.Base{policy.BaseAmount, policy.BaseFee},
		settle: func(e *evaluation, lines []chargeLine) decimal.Decimal {
			onFee, onAmount := decimal.Zero, decimal.Zero
			for _, line := range lines {
				if line.policy.Base == policy.BaseAmount {
					onAmount = onAmount.Add(line.amount)
				} else {
					onFee = onFee.Add(line.amount)
				}
			}
			fee := e.bases[policy.BaseFee]
			onFee = decimal.Min(onFee, fee)
			e.bases[policy.BaseNetFee] = fee.Sub(onFee)
			return onFee.Add(decimal.Min(onAmount, e.amount))
		},
	}
	// taxStrategy: paid, on any base.
	taxStrategy = chargeStrategy{
		of:    policy.KindTax,
		bases: []policy.Base{policy.BaseAmount, policy.BaseFee, policy.BaseNetFee},
	}
)

func (c chargeStrategy) kind() policy.Kind { return c.of }

func (c chargeStrategy) check(spec policy.Spec) error {
	return checkRules(spec,
		forbids(c.of, fieldOperator, ""),
		specRule{
			broken:  func(s policy.Spec) bool { return s.Base != "" && !slices.Contains(c.bases, s.Base) },
			message: fmt.Sprintf("%s policies take base %v or none (flat)", c.of, c.bases),
		},
		perTransaction(forbids(c.of, fieldUnit, " per transaction: the cap is money")),
		accumulating(requires(c.of, fieldCap, " with a DAILY, MONTHLY or LIFETIME window: what is charged per window")),
		accumulating(requires(c.of, fieldUnit, " with a DAILY, MONTHLY or LIFETIME window: what the cap counts")),
		accumulating(specRule{
			broken:  func(s policy.Spec) bool { _, ok := measureOf(s.Unit); return s.Unit != "" && !ok },
			message: fmt.Sprintf("a window cap counts one of %v", measured()),
		}),
	)
}

// process runs the template for the kind's policies; the kind's total is
// published even when it has none, so later kinds can use its base.
func (c chargeStrategy) process(e *evaluation, policies []policy.Policy) error {
	lines := make([]chargeLine, 0, len(policies))
	for _, p := range policies {
		amount, err := price(e, p)
		if err != nil {
			return err
		}
		if c.adjust != nil {
			amount = c.adjust(e, amount)
		}
		applied := appliedPolicy(p)
		amount = capped(e, p, amount, &applied)
		rounded := amount.Round(moneyPlaces)
		applied.Charge = &rounded
		e.apply(applied)
		lines = append(lines, chargeLine{policy: p, amount: amount})
	}
	total := decimal.Zero
	if c.settle != nil {
		total = c.settle(e, lines)
	} else {
		for _, line := range lines {
			total = total.Add(line.amount)
		}
	}
	if c.publish != nil {
		c.publish(e, total)
	}
	if len(lines) > 0 {
		e.result.Charges = append(e.result.Charges, Charge{Kind: c.of, Amount: total.Round(moneyPlaces)})
	}
	return nil
}

// price is a policy's flat value, or a percentage of its base.
func price(e *evaluation, p policy.Policy) (decimal.Decimal, error) {
	if p.Base == "" {
		return p.Value, nil
	}
	base, ok := e.bases[p.Base]
	if !ok {
		return decimal.Zero, errorf(p, "base %s is not computed before %s", p.Base, p.Kind)
	}
	return base.Mul(p.Value).Div(hundred), nil
}

// capped limits a line by its cap and records what it used of it. Over a
// TRANSACTION window the line is at most the cap; over an accumulating one,
// what the window has charged is at most the cap, counted in the unit.
// Without a cap the line is charged in full.
func capped(e *evaluation, p policy.Policy, amount decimal.Decimal, applied *AppliedPolicy) decimal.Decimal {
	if p.Cap == nil || amount.IsZero() {
		return amount
	}
	m := capMeasure(p)
	amount = m.fit(amount, p.Cap.Sub(e.used(p)))
	delta := decimal.Zero
	if amount.IsPositive() {
		delta = m.add(amount)
	}
	e.track(p, delta, *p.Cap, applied)
	return amount
}

// capMeasure is how a cap counts a line: in the window's unit when it
// accumulates, otherwise as money.
func capMeasure(p policy.Policy) measure {
	if m, ok := measureOf(p.Unit); ok && tracked(p) {
		return m
	}
	return amountMeasure{}
}
