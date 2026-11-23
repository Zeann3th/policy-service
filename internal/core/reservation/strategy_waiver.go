package reservation

import "github.com/zeann3th/policy-service/internal/core/policy"

// waiverStrategy waives the fee while the window's count is below the value
// ("the first N transfers a month are free"), or always without a window.
// Every covered transaction counts, waived or not.
type waiverStrategy struct{}

func (waiverStrategy) kind() policy.Kind { return policy.KindWaiver }

func (waiverStrategy) process(e *evaluation, policies []policy.Policy) error {
	for _, p := range policies {
		applied := appliedPolicy(p)
		if p.WindowType == policy.WindowNone {
			e.waived = true
			e.apply(applied)
			continue
		}
		current, err := e.track(p, &applied)
		if err != nil {
			return err
		}
		if current.LessThan(p.Value) {
			e.waived = true
		}
		e.apply(applied)
	}
	return nil
}
