package grpc

import (
	"testing"

	"github.com/zeann3th/policy-service/internal/core/policy"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPolicyMappingsExposeBusinessSelectors(t *testing.T) {
	typeMessage := outPolicyType(policy.PolicyTypeResponse{Kind: policy.KindFee, WindowType: policy.WindowDaily})
	policyMessage := outPolicy(policy.PolicyResponse{Direction: policy.DirectionOut})
	customMessage := outCustomPolicy(policy.CustomPolicyResponse{Direction: policy.DirectionIn})

	if typeMessage.Kind != "FEE" || typeMessage.WindowType != "DAILY" {
		t.Fatalf("policy type = %#v", typeMessage)
	}
	if policyMessage.Direction != "OUT" || customMessage.Direction != "IN" {
		t.Fatalf("directions = %q, %q", policyMessage.Direction, customMessage.Direction)
	}
}

func TestPolicyInputValidation(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "currency", err: isoCurrency("ZZZ")},
		{name: "kind", err: policyTypeSelectors("UNKNOWN", "NONE")},
		{name: "window", err: policyTypeSelectors("LIMIT", "YEARLY")},
		{name: "direction", err: direction("SIDEWAYS")},
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
