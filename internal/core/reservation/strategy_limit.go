package reservation

import (
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// limitStrategy marks a policy violated when its operator fails on what the
// transaction measures in the policy's unit (the amount, or 1 for COUNT),
// added to the window's usage when the window accumulates.
type limitStrategy struct{}

var limitRules = []specRule{
	requires(policy.KindLimit, fieldUnit, ": what is measured"),
	requires(policy.KindLimit, fieldOperator, ": how usage compares to the value"),
	forbids(policy.KindLimit, fieldBase, ""),
	forbids(policy.KindLimit, fieldCap, ": the value is the bound"),
}

func (limitStrategy) kind() policy.Kind { return policy.KindLimit }

func (limitStrategy) check(spec policy.Spec) error { return checkRules(spec, limitRules...) }

func (limitStrategy) process(e *evaluation, policies []policy.Policy) error {
	for _, p := range policies {
		m, ok := measureOf(p.Unit)
		if !ok {
			continue // a unit the code does not measure does nothing
		}
		applied := appliedPolicy(p)
		delta := m.add(e.amount)
		total := e.track(p, delta, p.Value, &applied).Add(delta)
		applied.Violated = !p.Operator.Satisfied(total, p.Value)
		e.apply(applied)
	}
	return nil
}
