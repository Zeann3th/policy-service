package reservation

import (
	"maps"
	"slices"

	"github.com/shopspring/decimal"
	"github.com/zeann3th/policy-service/internal/core/policy"
)

// measure is how a unit counts usage. Units are catalog rows; only the
// codes registered here are measured. A policy whose unit has no measure
// (BALANCE) is served by GET /v1/policies and enforced by another service.
type measure interface {
	// add is what a quantity q (the transaction amount, or an amount
	// charged) adds to the unit's usage.
	add(q decimal.Decimal) decimal.Decimal
	// fit is the part of q that still fits under a cap with room left.
	fit(q, room decimal.Decimal) decimal.Decimal
}

var measures = map[string]measure{
	policy.UnitAmount: amountMeasure{},
	policy.UnitCount:  countMeasure{},
}

// measured is every unit code with a measure, sorted.
func measured() []string {
	return slices.Sorted(maps.Keys(measures))
}

func measureOf(unit string) (measure, bool) {
	value, ok := measures[unit]
	return value, ok
}

// amountMeasure counts money: the quantity itself, partly when it does not
// all fit.
type amountMeasure struct{}

func (amountMeasure) add(q decimal.Decimal) decimal.Decimal { return q }

func (amountMeasure) fit(q, room decimal.Decimal) decimal.Decimal {
	return decimal.Max(decimal.Min(q, room), decimal.Zero)
}

// countMeasure counts transactions: one each, all or nothing.
type countMeasure struct{}

func (countMeasure) add(decimal.Decimal) decimal.Decimal { return one }

func (countMeasure) fit(q, room decimal.Decimal) decimal.Decimal {
	if room.GreaterThanOrEqual(one) {
		return q
	}
	return decimal.Zero
}
