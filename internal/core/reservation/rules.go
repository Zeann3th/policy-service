package reservation

import (
	"errors"
	"fmt"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// rules checks policies with the same strategies that evaluate them, so
// what can be written is exactly what can be evaluated.
type rules struct{}

// NewRules returns the policy.Rules of the registered strategies.
func NewRules() policy.Rules { return rules{} }

func (rules) Check(spec policy.Spec) error {
	value, ok := strategies.byKind[spec.Kind]
	if !ok {
		return fmt.Errorf("no strategy for kind %q", spec.Kind)
	}
	switch {
	case spec.Value.IsNegative():
		return errors.New("value cannot be negative: the policy type decides the sign")
	case spec.Cap != nil && spec.Cap.IsNegative():
		return errors.New("cap cannot be negative")
	case !spec.Window.Valid():
		return fmt.Errorf("unknown window %q", spec.Window)
	case spec.Operator != "" && !spec.Operator.Valid():
		return fmt.Errorf("unknown operator %q", spec.Operator)
	case spec.Base != "" && !spec.Base.Valid():
		return fmt.Errorf("unknown base %q", spec.Base)
	}
	return value.check(spec)
}
