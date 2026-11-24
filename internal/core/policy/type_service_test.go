package policy

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
)

// typeStore serves one policy type and the AMOUNT unit; the rest of
// PolicyStore is not used by check.
type typeStore struct {
	PolicyStore
	policyType PolicyType
}

func (s typeStore) GetType(context.Context, int64) (PolicyType, error) { return s.policyType, nil }

func (typeStore) GetUnitCode(context.Context, int64) (string, error) { return UnitAmount, nil }

type recordedRules struct{ spec *Spec }

func (r recordedRules) Check(spec Spec) error { *r.spec = spec; return nil }

func TestCheckTakesKindFromTheType(t *testing.T) {
	var got Spec
	s := &service{store: typeStore{policyType: PolicyType{Kind: KindDiscount}}, rules: recordedRules{spec: &got}}
	value := decimal.NewFromInt(10)
	if err := s.check(context.Background(), 1, nil, PolicyChange{Base: BaseFee, Value: &value}.spec(WindowTransaction)); err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindDiscount || got.Base != BaseFee {
		t.Fatalf("spec = %#v, want the type's kind with the policy's base", got)
	}
}
