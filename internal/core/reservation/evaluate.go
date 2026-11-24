package reservation

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

var (
	one     = decimal.NewFromInt(1)
	hundred = decimal.NewFromInt(100)
)

const moneyPlaces = 4

// tracked reports whether a policy accumulates usage: it is enforced, its
// window adds up and its unit is one this service measures.
func tracked(p policy.Policy) bool {
	_, measured := measureOf(p.Unit)
	return p.IsEnforced && accumulates(p.WindowType) && p.UnitID != nil && measured
}

func usageKey(walletID string, p policy.Policy, at time.Time) UsageKey {
	var unitID int64
	if p.UnitID != nil {
		unitID = *p.UnitID
	}
	return UsageKey{
		WalletID:        walletID,
		PolicyTypeID:    p.PolicyTypeID,
		TransactionType: p.TransactionType,
		Channel:         p.Channel,
		Direction:       p.Direction,
		WindowType:      p.WindowType,
		UnitID:          unitID,
		WindowStart:     windowStart(p.WindowType, at),
	}
}

func appliedPolicy(p policy.Policy) AppliedPolicy {
	return AppliedPolicy{
		PolicyID:        p.ID,
		PolicyType:      p.PolicyType,
		Kind:            p.Kind,
		Unit:            p.Unit,
		Base:            p.Base,
		Source:          p.Source,
		TransactionType: p.TransactionType,
		Channel:         p.Channel,
		Direction:       p.Direction,
		WindowType:      p.WindowType,
		Operator:        p.Operator,
		Value:           p.Value,
		Cap:             p.Cap,
	}
}

// evaluation is one side's state while the strategies process its policies.
// A strategy reads what earlier strategies left here (waived, bases) and
// adds its own results.
type evaluation struct {
	walletID string
	amount   decimal.Decimal
	at       time.Time
	usage    map[UsageKey]decimal.Decimal
	// bases holds the charge bases computed so far, by the charge types
	// that own them: AMOUNT at the start, FEE after the fee, NET_FEE after
	// the discount.
	bases   map[policy.Base]decimal.Decimal
	waived  bool
	result  PartyResult
	entries []LedgerEntry
}

// apply records a policy's result once it is final.
func (e *evaluation) apply(applied AppliedPolicy) {
	e.result.AppliedPolicies = append(e.result.AppliedPolicies, applied)
}

// used is a policy's usage before this transaction: its window total, or
// zero when it keeps none (a TRANSACTION window looks at the transaction
// alone).
func (e *evaluation) used(p policy.Policy) decimal.Decimal {
	if !tracked(p) {
		return decimal.Zero
	}
	return e.usage[usageKey(e.walletID, p, e.at)]
}

// track adds delta to a tracked policy's window and records the RESERVE
// entry; bound is the policy's limit (its value, or its cap for a charge).
// It returns the usage before this transaction. A policy that keeps no
// usage records nothing, so every strategy can call it alike.
func (e *evaluation) track(p policy.Policy, delta, bound decimal.Decimal, applied *AppliedPolicy) decimal.Decimal {
	if !tracked(p) {
		return decimal.Zero
	}
	key := usageKey(e.walletID, p, e.at)
	current := e.usage[key]
	total := current.Add(delta)
	applied.WindowStart = key.WindowStart.Format(time.DateOnly)
	applied.Usage = &total
	var operator *policy.Operator
	if p.Operator != "" {
		operator = &p.Operator
	}
	if !delta.IsZero() {
		e.entries = append(e.entries, LedgerEntry{
			EntryType:      EntryReserve,
			UsageKey:       key,
			Delta:          delta,
			PolicyID:       &p.ID,
			PolicyTypeCode: p.PolicyType,
			Operator:       operator,
			LimitValue:     &bound,
		})
	}
	return current
}

// evaluate applies one side's policies to a transaction of amount at time at.
// usage holds the current window totals of the side's tracked policies. Each
// kind is handled by its strategy (see strategies). It returns the side's
// result and the RESERVE entries to record if the reservation goes through.
func evaluate(walletID string, amount decimal.Decimal, policies []policy.Policy, usage map[UsageKey]decimal.Decimal, at time.Time) (PartyResult, []LedgerEntry, error) {
	e := &evaluation{
		walletID: walletID,
		amount:   amount,
		at:       at,
		usage:    usage,
		bases:    map[policy.Base]decimal.Decimal{policy.BaseAmount: amount},
		result:   PartyResult{WalletID: walletID, Charges: []Charge{}, AppliedPolicies: make([]AppliedPolicy, 0, len(policies))},
	}
	if err := strategies.process(e, policies); err != nil {
		return PartyResult{}, nil, err
	}
	return e.result, e.entries, nil
}

// errorf prefixes an evaluation error with the policy it is about.
func errorf(p policy.Policy, format string, args ...any) error {
	return fmt.Errorf("policy %d (%s): %s", p.ID, p.PolicyType, fmt.Sprintf(format, args...))
}
