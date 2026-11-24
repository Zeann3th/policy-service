package reservation

import (
	"errors"
	"fmt"
	"slices"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// chargeType is the behaviour of one AUXILIARY role. The auxiliary
// strategy is the template: for every policy of the role it prices the
// line on its base, lets adjust change it, applies the policy's window cap
// and records the usage; then settle turns the lines into the type's total
// and publish makes bases available to later types. A nil hook keeps the
// default.
type chargeType struct {
	role   policy.Role
	bases  []policy.Base // what its policies may be a percentage of
	credit bool          // taken off what the wallet pays (a discount)
	// adjust changes a priced line before the cap; default: unchanged.
	adjust func(e *evaluation, amount decimal.Decimal) decimal.Decimal
	// settle is the type's total from its capped lines; default: their sum.
	settle func(e *evaluation, lines []chargeLine) decimal.Decimal
	// publish makes the total available as a base; default: nothing.
	publish func(e *evaluation, total decimal.Decimal)
}

type chargeLine struct {
	policy policy.Policy
	amount decimal.Decimal
}

// chargeTypes runs in role order; a role may only use bases published by
// an earlier one.
var chargeTypes = []chargeType{
	{
		role:  policy.RoleFee,
		bases: []policy.Base{policy.BaseAmount},
		adjust: func(e *evaluation, amount decimal.Decimal) decimal.Decimal {
			if e.waived {
				return decimal.Zero
			}
			return amount
		},
		publish: func(e *evaluation, total decimal.Decimal) { e.bases[policy.BaseFee] = total },
	},
	{
		role:   policy.RoleDiscount,
		bases:  []policy.Base{policy.BaseAmount, policy.BaseFee},
		credit: true,
		// flat and fee-based discounts come off the fee (at most the fee);
		// amount-based ones (cashback) at most the amount
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
	},
	{
		role:  policy.RoleTax,
		bases: []policy.Base{policy.BaseAmount, policy.BaseFee, policy.BaseNetFee},
	},
}

func chargeTypeOf(role policy.Role) (chargeType, bool) {
	index := slices.IndexFunc(chargeTypes, func(value chargeType) bool { return value.role == role })
	if index < 0 {
		return chargeType{}, false
	}
	return chargeTypes[index], true
}

// auxiliaryStrategy charges fees, discounts, taxes and any other AUXILIARY
// type registered in chargeTypes.
type auxiliaryStrategy struct{}

func (auxiliaryStrategy) kind() policy.Kind { return policy.KindAuxiliary }

func (auxiliaryStrategy) check(spec policy.Spec) error {
	value, ok := chargeTypeOf(spec.Role)
	if !ok {
		return fmt.Errorf("AUXILIARY policies need a role %v", policy.Roles())
	}
	allowed := value.bases
	switch {
	case spec.Operator != "":
		return errors.New("AUXILIARY policies take no operator")
	case spec.Base != "" && !slices.Contains(allowed, spec.Base):
		return fmt.Errorf("%s policies take base %v or none (flat)", spec.Role, allowed)
	case spec.Window == policy.WindowTransaction && spec.Cap != nil:
		return errors.New("a cap needs a DAILY, MONTHLY or LIFETIME window: it limits what is charged per window")
	case spec.Window == policy.WindowTransaction && spec.Unit != "":
		return errors.New("AUXILIARY policies take a unit only with a capped window (what the cap counts)")
	case spec.Window != policy.WindowTransaction && spec.Cap == nil:
		return errors.New("an AUXILIARY policy with a DAILY, MONTHLY or LIFETIME window needs a cap")
	case spec.Window != policy.WindowTransaction && spec.Unit == "":
		return errors.New("an AUXILIARY policy with a DAILY, MONTHLY or LIFETIME window needs a unit for its cap")
	}
	return nil
}

func (auxiliaryStrategy) process(e *evaluation, policies []policy.Policy) error {
	byRole := make(map[policy.Role][]policy.Policy, len(chargeTypes))
	for _, p := range policies {
		byRole[p.Role] = append(byRole[p.Role], p)
	}
	for _, value := range chargeTypes {
		if err := charge(e, value, byRole[value.role]); err != nil {
			return err
		}
	}
	return nil
}

// charge runs the template for one charge type.
func charge(e *evaluation, value chargeType, policies []policy.Policy) error {
	lines := make([]chargeLine, 0, len(policies))
	for _, p := range policies {
		amount, err := price(e, p)
		if err != nil {
			return err
		}
		if value.adjust != nil {
			amount = value.adjust(e, amount)
		}
		applied := appliedPolicy(p)
		amount = capped(e, p, amount, &applied)
		signed := amount.Round(moneyPlaces)
		if value.credit {
			signed = signed.Neg()
		}
		applied.Charge = &signed
		e.apply(applied)
		lines = append(lines, chargeLine{policy: p, amount: amount})
	}
	total := decimal.Zero
	if value.settle != nil {
		total = value.settle(e, lines)
	} else {
		for _, line := range lines {
			total = total.Add(line.amount)
		}
	}
	if value.publish != nil {
		value.publish(e, total)
	}
	if len(lines) > 0 {
		if value.credit {
			total = total.Neg()
		}
		e.result.Charges = append(e.result.Charges, Charge{Role: value.role, Amount: total.Round(moneyPlaces)})
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
		return decimal.Zero, errorf(p, "base %s is not computed before %s", p.Base, p.Role)
	}
	return base.Mul(p.Value).Div(hundred), nil
}

// capped limits a line by its window cap and records what it used of it.
// Without a window (or a cap) the line is charged in full.
func capped(e *evaluation, p policy.Policy, amount decimal.Decimal, applied *AppliedPolicy) decimal.Decimal {
	m, ok := measureOf(p.Unit)
	if !tracked(p) || !ok || p.Cap == nil || amount.IsZero() {
		return amount
	}
	used := e.usage[usageKey(e.walletID, p, e.at)]
	amount = m.fit(amount, p.Cap.Sub(used))
	delta := decimal.Zero
	if amount.IsPositive() {
		delta = m.add(amount)
	}
	e.track(p, delta, *p.Cap, applied)
	return amount
}
