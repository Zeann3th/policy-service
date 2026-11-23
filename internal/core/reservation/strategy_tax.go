package reservation

import "github.com/zeann3th/policy-service/internal/core/policy"

// taxStrategy sums the taxes: flat, or a percentage of AMOUNT, FEE or
// NET_FEE.
type taxStrategy struct{}

func (taxStrategy) kind() policy.Kind { return policy.KindTax }

func (taxStrategy) process(e *evaluation, policies []policy.Policy) error {
	tax, err := e.sum(policies)
	if err != nil {
		return err
	}
	e.result.Tax = tax.Round(moneyPlaces)
	return nil
}
