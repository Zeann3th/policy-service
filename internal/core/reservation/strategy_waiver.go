package reservation

import (
	"errors"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// waiverStrategy waives the fee while the window's usage, in the policy's
// unit, is below the value ("the first 5 transfers a month are free"), or
// always with window NONE. Every covered transaction counts, waived or not.
type waiverStrategy struct{}

func (waiverStrategy) kind() policy.Kind { return policy.KindWaiver }

func (waiverStrategy) check(spec policy.Spec) error {
	switch {
	case spec.Unit == "":
		return errors.New("WAIVER policies need a unit")
	case spec.Operator != "":
		return errors.New("WAIVER policies take no operator: free while usage < value")
	case spec.Base != "":
		return errors.New("WAIVER policies take no base")
	case spec.Cap != nil:
		return errors.New("WAIVER policies take no cap: the value is the bound")
	}
	return nil
}

func (waiverStrategy) process(e *evaluation, policies []policy.Policy) error {
	for _, p := range policies {
		m, ok := measureOf(p.Unit)
		if !ok {
			continue // a unit the code does not measure does nothing
		}
		applied := appliedPolicy(p)
		if p.WindowType == policy.WindowNone {
			e.waived = true
			e.apply(applied)
			continue
		}
		if e.track(p, m.add(e.amount), p.Value, &applied).LessThan(p.Value) {
			e.waived = true
		}
		e.apply(applied)
	}
	return nil
}
