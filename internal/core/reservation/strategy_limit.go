package reservation

import "github.com/zeann3th/policy-service/internal/core/policy"

// limitStrategy marks a policy violated when its operator fails on the
// transaction's contribution (the amount, or 1 for COUNT), or with a window
// on the usage including it. BALANCE limits are skipped: the request carries
// no balance.
type limitStrategy struct{}

func (limitStrategy) kind() policy.Kind { return policy.KindLimit }

func (limitStrategy) process(e *evaluation, policies []policy.Policy) error {
	for _, p := range policies {
		if p.Unit == policy.UnitBalance {
			continue
		}
		applied := appliedPolicy(p)
		actual, err := contribution(p, e.amount)
		if err != nil {
			return err
		}
		if tracked(p) {
			if _, err := e.track(p, &applied); err != nil {
				return err
			}
			actual = *applied.Usage
		}
		applied.Violated = !p.Operator.Satisfied(actual, p.Value)
		e.apply(applied)
	}
	return nil
}
