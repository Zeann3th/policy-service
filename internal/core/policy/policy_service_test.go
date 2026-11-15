package policy

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestMergePoliciesOverridesOnlyExactSelector(t *testing.T) {
	system := []Policy{
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionAll, Value: decimal.NewFromInt(100)},
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionOut, Value: decimal.NewFromInt(200)},
	}
	custom := []CustomPolicy{
		{PolicyTypeID: 1, TransactionType: All, Channel: All, Direction: DirectionOut, Value: decimal.NewFromInt(50)},
		{PolicyTypeID: 1, TransactionType: "TRANSFER", Channel: All, Direction: DirectionOut, Value: decimal.NewFromInt(25)},
	}

	got := mergePolicies(system, custom)

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if !got[0].Value.Equal(decimal.NewFromInt(100)) || got[0].Source != SourceSystem {
		t.Fatalf("ALL direction changed: %#v", got[0])
	}
	if !got[1].Value.Equal(decimal.NewFromInt(50)) || got[1].Source != SourceCustom {
		t.Fatalf("OUT direction not overridden: %#v", got[1])
	}
	if got[2].TransactionType != "TRANSFER" || got[2].Source != SourceCustom {
		t.Fatalf("specific selector not appended: %#v", got[2])
	}
}

func TestFilterPoliciesKeepsGlobalAndMatchingTransaction(t *testing.T) {
	values := []Policy{{TransactionType: All}, {TransactionType: "TRANSFER"}, {TransactionType: "PAYMENT"}}
	got := filterPoliciesByTransactionType(values, "TRANSFER")
	if len(got) != 2 || got[0].TransactionType != All || got[1].TransactionType != "TRANSFER" {
		t.Fatalf("filtered = %#v", got)
	}
}
