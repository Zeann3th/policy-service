package reservation

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
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

// tracked reports whether a policy accumulates usage: a windowed policy
// that measures AMOUNT or COUNT.
func tracked(p policy.Policy) bool {
	return p.WindowType != policy.WindowNone && (p.Unit == policy.UnitAmount || p.Unit == policy.UnitCount)
}

func usageKey(walletID string, p policy.Policy, at time.Time) UsageKey {
	return UsageKey{
		WalletID:        walletID,
		PolicyTypeID:    p.PolicyTypeID,
		TransactionType: p.TransactionType,
		Channel:         p.Channel,
		Direction:       p.Direction,
		WindowType:      p.WindowType,
		Unit:            p.Unit,
		WindowStart:     windowStart(p.WindowType, at),
	}
}

// contribution is what one transaction adds to a policy's measure.
func contribution(p policy.Policy, amount decimal.Decimal) (decimal.Decimal, error) {
	switch p.Unit {
	case policy.UnitAmount:
		return amount, nil
	case policy.UnitCount:
		return one, nil
	}
	return decimal.Zero, fmt.Errorf("policy %d (%s): unit %q cannot be measured", p.ID, p.PolicyType, p.Unit)
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
	// bases holds the charge bases computed so far, by the strategies that
	// own them: AMOUNT at the start, FEE after FEE, NET_FEE after DISCOUNT.
	bases   map[policy.Base]decimal.Decimal
	waived  bool
	result  PartyResult
	entries []LedgerEntry
}

func (e *evaluation) apply(applied AppliedPolicy) {
	e.result.IsViolated = e.result.IsViolated || applied.Violated
	e.result.AppliedPolicies = append(e.result.AppliedPolicies, applied)
}

// track adds a windowed policy's contribution to its usage and records the
// RESERVE entry. It returns the usage before this transaction.
func (e *evaluation) track(p policy.Policy, applied *AppliedPolicy) (decimal.Decimal, error) {
	delta, err := contribution(p, e.amount)
	if err != nil {
		return decimal.Zero, err
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
	e.entries = append(e.entries, LedgerEntry{
		EntryType:      EntryReserve,
		UsageKey:       key,
		Delta:          delta,
		PolicyID:       &p.ID,
		PolicyTypeCode: p.PolicyType,
		Operator:       operator,
		LimitValue:     &p.Value,
	})
	return current, nil
}

// charge is the policy's flat value, or a percentage of its base.
func (e *evaluation) charge(p policy.Policy) (decimal.Decimal, error) {
	if p.Base == "" {
		return p.Value, nil
	}
	base, ok := e.bases[p.Base]
	if !ok {
		return decimal.Zero, fmt.Errorf("policy %d (%s): base %s is not computed before %s", p.ID, p.PolicyType, p.Base, p.Kind)
	}
	return base.Mul(p.Value).Div(hundred), nil
}

// sum applies policies as charges and returns their total.
func (e *evaluation) sum(policies []policy.Policy) (decimal.Decimal, error) {
	total := decimal.Zero
	for _, p := range policies {
		value, err := e.charge(p)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(value)
		applied := appliedPolicy(p)
		charge := value.Round(moneyPlaces)
		applied.Charge = &charge
		e.apply(applied)
	}
	return total, nil
}

// UsageKeys returns the usage window each tracked policy reads at time at,
// by policy id.
func UsageKeys(walletID string, policies []policy.Policy, at time.Time) map[int64]UsageKey {
	keys := make(map[int64]UsageKey)
	for _, p := range policies {
		if tracked(p) {
			keys[p.ID] = usageKey(walletID, p, at)
		}
	}
	return keys
}

// Preview evaluates one side without touching storage; usage holds the
// window totals before this transaction. It is what Reserve would decide.
func Preview(walletID string, amount decimal.Decimal, policies []policy.Policy, usage map[UsageKey]decimal.Decimal, at time.Time) (PartyResult, error) {
	result, _, err := evaluate(walletID, amount, policies, usage, at)
	return result, err
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
		result:   PartyResult{WalletID: walletID, Fee: decimal.Zero, Discount: decimal.Zero, Tax: decimal.Zero, AppliedPolicies: make([]AppliedPolicy, 0, len(policies))},
	}
	if err := strategies.process(e, policies); err != nil {
		return PartyResult{}, nil, err
	}
	return e.result, e.entries, nil
}
