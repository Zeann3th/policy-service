package reservation

import (
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// feeStrategy sums the fees (flat, or a percentage of AMOUNT); zero when a
// waiver applies. Publishes base FEE.
type feeStrategy struct{}

func (feeStrategy) kind() policy.Kind { return policy.KindFee }

func (feeStrategy) process(e *evaluation, policies []policy.Policy) error {
	fee, err := e.sum(policies)
	if err != nil {
		return err
	}
	if e.waived {
		fee = decimal.Zero
	}
	e.bases[policy.BaseFee] = fee
	e.result.Fee = fee.Round(moneyPlaces)
	return nil
}
