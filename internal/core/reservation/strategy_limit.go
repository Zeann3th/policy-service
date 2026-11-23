package reservation

import (
	"errors"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// limitStrategy marks a policy violated when its operator fails on what the
// transaction measures in the policy's unit (the amount, or 1 for COUNT),
// or with a window on the usage including it.
type limitStrategy struct{}

func (limitStrategy) kind() policy.Kind { return policy.KindLimit }

func (limitStrategy) check(spec policy.Spec) error {
	switch {
	case spec.Unit == "":
		return errors.New("LIMIT policies need a unit")
	case spec.Operator == "":
		return errors.New("LIMIT policies need an operator")
	case spec.Base != "":
		return errors.New("LIMIT policies take no base")
	case spec.Cap != nil:
		return errors.New("LIMIT policies take no cap: the value is the bound")
	}
	return nil
}

func (limitStrategy) process(e *evaluation, policies []policy.Policy) error {
	for _, p := range policies {
		m, ok := measureOf(p.Unit)
		if !ok {
			continue // a unit the code does not measure does nothing
		}
		applied := appliedPolicy(p)
		actual := m.add(e.amount)
		if tracked(p) {
			actual = e.track(p, actual, p.Value, &applied).Add(actual)
		}
		applied.Violated = !p.Operator.Satisfied(actual, p.Value)
		e.apply(applied)
	}
	return nil
}
