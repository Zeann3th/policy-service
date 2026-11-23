package reservation

import (
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// feeStrategy sums the fees (flat, or a percentage of AMOUNT); zero, line by
// line, when a waiver applies. Publishes base FEE.
type feeStrategy struct{}

func (feeStrategy) kind() policy.Kind { return policy.KindFee }

func (feeStrategy) process(e *evaluation, policies []policy.Policy) error {
	fee, err := e.sum(policies)
	if err != nil {
		return err
	}
	if e.waived {
		fee = decimal.Zero
		for i := len(e.result.AppliedPolicies) - len(policies); i < len(e.result.AppliedPolicies); i++ {
			e.result.AppliedPolicies[i].Charge = &fee
		}
	}
	e.bases[policy.BaseFee] = fee
	e.result.Fee = fee.Round(moneyPlaces)
	return nil
}
