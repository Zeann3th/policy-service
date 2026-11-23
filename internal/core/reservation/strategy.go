package reservation

import (
	"fmt"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// strategy evaluates all policies of one kind for one side of a
// transaction. It is called even when the side has none of its policies, so
// it can publish its results (for example a zero fee) to later strategies.
type strategy interface {
	kind() policy.Kind
	process(e *evaluation, policies []policy.Policy) error
}

// registry runs strategies in registration order. A strategy may only read
// what an earlier one computed, so the order is part of the business rules.
type registry struct {
	order []strategy
	kinds map[policy.Kind]struct{}
}

func newRegistry(values ...strategy) registry {
	r := registry{kinds: make(map[policy.Kind]struct{}, len(values))}
	for _, value := range values {
		if _, ok := r.kinds[value.kind()]; ok {
			panic(fmt.Sprintf("strategy for kind %s registered twice", value.kind()))
		}
		r.kinds[value.kind()] = struct{}{}
		r.order = append(r.order, value)
	}
	return r
}

// strategies is the processing order of a reservation side: limits and the
// waiver first, then the charges, each charge after the bases it reads.
// To add a kind: declare it with its shape in package policy, write its
// strategy, and register it here at the right place.
var strategies = newRegistry(
	limitStrategy{},
	waiverStrategy{},
	feeStrategy{},
	discountStrategy{},
	taxStrategy{},
)

func (r registry) process(e *evaluation, policies []policy.Policy) error {
	byKind := make(map[policy.Kind][]policy.Policy, len(r.order))
	for _, p := range policies {
		if _, ok := r.kinds[p.Kind]; !ok {
			return fmt.Errorf("policy %d (%s): no strategy for kind %q", p.ID, p.PolicyType, p.Kind)
		}
		byKind[p.Kind] = append(byKind[p.Kind], p)
	}
	for _, value := range r.order {
		if err := value.process(e, byKind[value.kind()]); err != nil {
			return err
		}
	}
	return nil
}
