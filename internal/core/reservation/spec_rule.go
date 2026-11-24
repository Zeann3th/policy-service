package reservation

import (
	"errors"
	"fmt"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

// specRule is one thing a policy must satisfy to be written. Each kind
// declares its rules as a list (see the strategies' rules), checked in
// order; the first broken one is the error.
type specRule struct {
	broken  func(policy.Spec) bool
	message string
}

func checkRules(spec policy.Spec, rules ...specRule) error {
	for _, rule := range rules {
		if rule.broken(spec) {
			return errors.New(rule.message)
		}
	}
	return nil
}

// field is a value field a kind takes or not.
type field struct {
	article, name string
	set           func(policy.Spec) bool
}

var (
	fieldUnit     = field{"a", "unit", func(s policy.Spec) bool { return s.Unit != "" }}
	fieldOperator = field{"an", "operator", func(s policy.Spec) bool { return s.Operator != "" }}
	fieldBase     = field{"a", "base", func(s policy.Spec) bool { return s.Base != "" }}
	fieldCap      = field{"a", "cap", func(s policy.Spec) bool { return s.Cap != nil }}
)

// requires breaks when the field is missing; why says what it is for.
func requires(kind policy.Kind, f field, why string) specRule {
	return specRule{
		broken:  func(s policy.Spec) bool { return !f.set(s) },
		message: fmt.Sprintf("%s policies need %s %s%s", kind, f.article, f.name, why),
	}
}

// forbids breaks when the field is set; why says what to use instead.
func forbids(kind policy.Kind, f field, why string) specRule {
	return specRule{
		broken:  f.set,
		message: fmt.Sprintf("%s policies take no %s%s", kind, f.name, why),
	}
}

// accumulating narrows a rule to policies whose window adds up usage.
func accumulating(rule specRule) specRule {
	return specRule{
		broken:  func(s policy.Spec) bool { return accumulates(s.Window) && rule.broken(s) },
		message: rule.message,
	}
}

// perTransaction narrows a rule to policies over a TRANSACTION window.
func perTransaction(rule specRule) specRule {
	return specRule{
		broken:  func(s policy.Spec) bool { return !accumulates(s.Window) && rule.broken(s) },
		message: rule.message,
	}
}
