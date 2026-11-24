package reservation

import (
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// waiverStrategy waives the fee while the usage before the transaction, in
// the policy's unit, is below the value ("the first 5 transfers a month
// are free"). Over a TRANSACTION window there is no prior usage, so any
// positive value waives every fee. Every covered transaction counts,
// waived or not.
type waiverStrategy struct{}

var waiverRules = []specRule{
	requires(policy.KindWaiver, fieldUnit, ": what is counted"),
	forbids(policy.KindWaiver, fieldOperator, ": free while usage < value"),
	forbids(policy.KindWaiver, fieldBase, ""),
	forbids(policy.KindWaiver, fieldCap, ": the value is the bound"),
}

func (waiverStrategy) kind() policy.Kind { return policy.KindWaiver }

func (waiverStrategy) check(spec policy.Spec) error { return checkRules(spec, waiverRules...) }

func (waiverStrategy) process(e *evaluation, policies []policy.Policy) error {
	for _, p := range policies {
		m, ok := measureOf(p.Unit)
		if !ok {
			continue // a unit the code does not measure does nothing
		}
		applied := appliedPolicy(p)
		before := e.track(p, m.add(e.amount), p.Value, &applied)
		e.waived = e.waived || before.LessThan(p.Value)
		e.apply(applied)
	}
	return nil
}
