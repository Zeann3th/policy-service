package reservation

import (
	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// discountStrategy sums the discounts. Flat and FEE-based discounts come off
// the fee and are capped at it; AMOUNT-based ones (cashback) are capped at
// the amount. The result reports both together. Publishes base NET_FEE: the
// fee minus the fee discounts.
type discountStrategy struct{}

func (discountStrategy) kind() policy.Kind { return policy.KindDiscount }

func (discountStrategy) process(e *evaluation, policies []policy.Policy) error {
	var onFee, onAmount []policy.Policy
	for _, p := range policies {
		if p.Base == policy.BaseAmount {
			onAmount = append(onAmount, p)
		} else {
			onFee = append(onFee, p)
		}
	}
	fee := e.bases[policy.BaseFee]
	feeDiscount, err := e.sum(onFee)
	if err != nil {
		return err
	}
	feeDiscount = decimal.Min(feeDiscount, fee)
	amountDiscount, err := e.sum(onAmount)
	if err != nil {
		return err
	}
	amountDiscount = decimal.Min(amountDiscount, e.amount)
	e.bases[policy.BaseNetFee] = fee.Sub(feeDiscount)
	e.result.Discount = feeDiscount.Add(amountDiscount).Round(moneyPlaces)
	return nil
}
