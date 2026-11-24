package reservation

import (
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// waiverStrategy waives the fee while every waiver covering the transaction
// still has room: its usage before the transaction, in its unit, is below
// its value ("the first 5 transfers a day are free, 3 of them TI0001"), so
// the strictest one wins. Over a TRANSACTION window there is no prior
// usage, so any positive value has room. Each waiver says whether it had
// room (Waived); a waiver never violates. Every covered transaction
// counts, waived or not.
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
	covered, room := false, true
	for _, p := range policies {
		m, ok := measureOf(p.Unit)
		if !ok {
			continue // a unit the code does not measure does nothing
		}
		applied := appliedPolicy(p)
		waived := e.track(p, m.add(e.amount), p.Value, &applied).LessThan(p.Value)
		applied.Waived = &waived
		covered, room = true, room && waived
		e.apply(applied)
	}
	e.waived = covered && room
	return nil
}
