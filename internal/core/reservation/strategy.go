package reservation

import (
	"fmt"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// strategy is the behaviour of one policy kind: which value fields its
// policies take (check, used when a policy is written) and how they are
// evaluated for one side of a transaction (process, called even when the
// side has none of them, so it can publish its results to later
// strategies).
type strategy interface {
	kind() policy.Kind
	check(policy.Spec) error
	process(e *evaluation, policies []policy.Policy) error
}

// registry runs strategies in registration order. A strategy may only read
// what an earlier one computed, so the order is part of the business rules.
type registry struct {
	order  []strategy
	byKind map[policy.Kind]strategy
}

func newRegistry(values ...strategy) registry {
	r := registry{byKind: make(map[policy.Kind]strategy, len(values))}
	for _, value := range values {
		if _, ok := r.byKind[value.kind()]; ok {
			panic(fmt.Sprintf("strategy for kind %s registered twice", value.kind()))
		}
		r.byKind[value.kind()] = value
		r.order = append(r.order, value)
	}
	return r
}

// strategies is the processing order of a reservation side: limits and the
// waiver first, then the charges. To add a kind: declare it in package
// policy, write its strategy, and register it here at the right place.
var strategies = newRegistry(
	limitStrategy{},
	waiverStrategy{},
	auxiliaryStrategy{},
)

// process evaluates each enforced policy with its kind's strategy. A
// policy whose type is not enforced is config only (returned by GET
// /v1/policies, e.g. a BALANCE limit for the accounting service), and a
// kind with no strategy does nothing: neither reaches the engine.
func (r registry) process(e *evaluation, policies []policy.Policy) error {
	byKind := make(map[policy.Kind][]policy.Policy, len(r.order))
	for _, p := range policies {
		if _, ok := r.byKind[p.Kind]; !ok || !p.IsEnforced {
			continue
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
