package grpc

import (
	"testing"

	"github.com/zeann3th/policy-service/internal/core/policy"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPolicyMappingsExposeBusinessSelectors(t *testing.T) {
	typeMessage := outPolicyType(policy.PolicyTypeResponse{Code: "TXN_FEE"})
	policyMessage := outPolicy(policy.PolicyResponse{Direction: policy.DirectionOut, WindowType: policy.WindowMonthly})
	customMessage := outCustomPolicy(policy.CustomPolicyResponse{Direction: policy.DirectionIn, WindowType: policy.WindowLifetime})

	if typeMessage.Code != "TXN_FEE" {
		t.Fatalf("policy type = %#v", typeMessage)
	}
	if policyMessage.Direction != "OUT" || policyMessage.WindowType != "MONTHLY" || customMessage.Direction != "IN" || customMessage.WindowType != "LIFETIME" {
		t.Fatalf("policy/custom selectors = %#v, %#v", policyMessage, customMessage)
	}
}

func TestPolicyInputValidation(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "currency", err: isoCurrency("ZZZ")},
		{name: "kind", err: enum[policy.Kind]("kind", "UNKNOWN")},
		{name: "window", err: enum[policy.WindowType]("windowType", "YEARLY")},
		{name: "direction", err: enum[policy.Direction]("direction", "SIDEWAYS")},
		{name: "charge kind", err: enum[policy.Kind]("kind", "AUXILIARY")},
		{name: "operator", err: enum[policy.Operator]("operator", "ABOUT")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if status.Code(test.err) != codes.InvalidArgument {
				t.Fatalf("error = %v", test.err)
			}
		})
	}

	if err := isoCurrency("VND"); err != nil {
		t.Fatalf("VND rejected: %v", err)
	}
}
