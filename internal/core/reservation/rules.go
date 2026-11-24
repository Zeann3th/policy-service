package reservation

import (
	"fmt"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// rules checks policies with the same strategies that evaluate them, so
// what can be written is exactly what can be evaluated.
type rules struct{}

// NewRules returns the policy.Rules of the registered strategies.
func NewRules() policy.Rules { return rules{} }

// commonRules hold for every kind; each strategy adds its own.
var commonRules = []specRule{
	{broken: func(s policy.Spec) bool { return s.Value.IsNegative() }, message: "value cannot be negative: the kind decides the sign"},
	{broken: func(s policy.Spec) bool { return s.Cap != nil && s.Cap.IsNegative() }, message: "cap cannot be negative"},
	{broken: func(s policy.Spec) bool { return !s.Window.Valid() }, message: fmt.Sprintf("window must be one of %v", policy.Windows())},
	{broken: func(s policy.Spec) bool { return s.Operator != "" && !s.Operator.Valid() }, message: fmt.Sprintf("operator must be one of %v", policy.Operators())},
	{broken: func(s policy.Spec) bool { return s.Base != "" && !s.Base.Valid() }, message: fmt.Sprintf("base must be one of %v", policy.Bases())},
}

func (rules) Check(spec policy.Spec) error {
	value, ok := strategies.byKind[spec.Kind]
	if !ok {
		return fmt.Errorf("no strategy for kind %q", spec.Kind)
	}
	if err := checkRules(spec, commonRules...); err != nil {
		return err
	}
	return value.check(spec)
}
