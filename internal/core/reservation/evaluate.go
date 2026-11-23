package reservation

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
	"github.com/zeann3th/policy-service/internal/core/unit"
)

var (
	lifetimeStart = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	one           = decimal.NewFromInt(1)
	hundred       = decimal.NewFromInt(100)
)

const moneyPlaces = 4

func windowStart(window policy.WindowType, at time.Time) time.Time {
	utc := at.UTC()
	switch window {
	case policy.WindowDaily:
		return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	case policy.WindowMonthly:
		return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return lifetimeStart
}

// balanceLimit reports a COIN_MIN/COIN_MAX style limit. The request carries
// no balance, so these are not evaluated during reserve.
func balanceLimit(p policy.Policy) bool {
	return p.Kind == policy.KindLimit && p.Unit == unit.CodeBalance
}

// tracked reports whether a policy accumulates usage in a window.
func tracked(p policy.Policy) bool {
	return p.WindowType != policy.WindowNone && !balanceLimit(p) &&
		(p.Kind == policy.KindLimit || p.Kind == policy.KindWaiver)
}

func usageKey(walletID string, p policy.Policy, at time.Time) UsageKey {
	return UsageKey{
		WalletID:        walletID,
		PolicyTypeID:    p.PolicyTypeID,
		TransactionType: p.TransactionType,
		Channel:         p.Channel,
		Direction:       p.Direction,
		WindowType:      p.WindowType,
		WindowStart:     windowStart(p.WindowType, at),
	}
}

// contribution is what one transaction adds to a policy's measure.
func contribution(p policy.Policy, amount decimal.Decimal) (decimal.Decimal, error) {
	switch p.Unit {
	case unit.CodeAmount:
		return amount, nil
	case unit.CodeCount:
		return one, nil
	}
	return decimal.Zero, fmt.Errorf("policy %d (%s): unit %s cannot be measured", p.ID, p.PolicyType, p.Unit)
}

// charge is a flat AMOUNT or a PERCENT of base.
func charge(p policy.Policy, base decimal.Decimal) (decimal.Decimal, error) {
	switch p.Unit {
	case unit.CodeAmount:
		return p.Value, nil
	case unit.CodePercent:
		return base.Mul(p.Value).Div(hundred), nil
	}
	return decimal.Zero, fmt.Errorf("policy %d (%s): unit %s cannot be charged", p.ID, p.PolicyType, p.Unit)
}

func sumCharges(policies []policy.Policy, base decimal.Decimal) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, p := range policies {
		value, err := charge(p, base)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(value)
	}
	return total, nil
}

func appliedPolicy(p policy.Policy) AppliedPolicy {
	return AppliedPolicy{
		PolicyID:        p.ID,
		PolicyType:      p.PolicyType,
		Kind:            p.Kind,
		Unit:            p.Unit,
		Source:          p.Source,
		TransactionType: p.TransactionType,
		Channel:         p.Channel,
		Direction:       p.Direction,
		WindowType:      p.WindowType,
		Operator:        p.Operator,
		Value:           p.Value,
	}
}

// evaluate applies one side's policies to a transaction of amount at time at.
// usage holds the current window totals of the side's tracked policies. It
// returns the side's result and the RESERVE entries to record if the
// reservation goes through.
//
//   - LIMIT: violated when the operator fails on the amount (or 1 for COUNT),
//     or on window usage plus that contribution for windowed limits.
//   - WAIVER: the fee is waived while the window count is below the value
//     (always, without a window).
//   - FEE: flat AMOUNT or PERCENT of the amount, summed; zero when waived.
//   - DISCOUNT: flat AMOUNT or PERCENT of the fee, capped at the fee.
//   - TAX: flat AMOUNT or PERCENT of the fee after discount.
func evaluate(walletID string, amount decimal.Decimal, policies []policy.Policy, usage map[UsageKey]decimal.Decimal, at time.Time) (PartyResult, []LedgerEntry, error) {
	result := PartyResult{WalletID: walletID, AppliedPolicies: make([]AppliedPolicy, 0, len(policies))}
	var entries []LedgerEntry
	var fees, discounts, taxes []policy.Policy
	waived := false

	for _, p := range policies {
		if balanceLimit(p) {
			continue
		}
		applied := appliedPolicy(p)
		switch {
		case tracked(p):
			delta, err := contribution(p, amount)
			if err != nil {
				return PartyResult{}, nil, err
			}
			key := usageKey(walletID, p, at)
			current := usage[key]
			total := current.Add(delta)
			applied.WindowStart = key.WindowStart.Format(time.DateOnly)
			applied.Usage = &total
			if p.Kind == policy.KindLimit {
				applied.Violated = !p.Operator.Satisfied(total, p.Value)
			} else if current.LessThan(p.Value) {
				waived = true
			}
			entries = append(entries, LedgerEntry{
				EntryType:      EntryReserve,
				UsageKey:       key,
				Delta:          delta,
				PolicyID:       &p.ID,
				PolicyTypeCode: p.PolicyType,
				Operator:       &p.Operator,
				LimitValue:     &p.Value,
			})
		case p.Kind == policy.KindLimit:
			actual, err := contribution(p, amount)
			if err != nil {
				return PartyResult{}, nil, err
			}
			applied.Violated = !p.Operator.Satisfied(actual, p.Value)
		case p.Kind == policy.KindWaiver:
			waived = true
		case p.Kind == policy.KindFee:
			fees = append(fees, p)
		case p.Kind == policy.KindDiscount:
			discounts = append(discounts, p)
		case p.Kind == policy.KindTax:
			taxes = append(taxes, p)
		default:
			return PartyResult{}, nil, fmt.Errorf("policy %d (%s): unknown kind %q", p.ID, p.PolicyType, p.Kind)
		}
		result.IsViolated = result.IsViolated || applied.Violated
		result.AppliedPolicies = append(result.AppliedPolicies, applied)
	}

	fee, err := sumCharges(fees, amount)
	if err != nil {
		return PartyResult{}, nil, err
	}
	if waived {
		fee = decimal.Zero
	}
	discount, err := sumCharges(discounts, fee)
	if err != nil {
		return PartyResult{}, nil, err
	}
	discount = decimal.Min(discount, fee)
	tax, err := sumCharges(taxes, fee.Sub(discount))
	if err != nil {
		return PartyResult{}, nil, err
	}
	result.Fee = fee.Round(moneyPlaces)
	result.Discount = discount.Round(moneyPlaces)
	result.Tax = tax.Round(moneyPlaces)
	return result, entries, nil
}
