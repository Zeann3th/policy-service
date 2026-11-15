package limit

import (
	"encoding/json"
	"testing"
)

func TestGetLimitConfigResponseJSONContract(t *testing.T) {
	encoded, err := json.Marshal(GetLimitConfigResponse{Rules: []LimitRule{{}}})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var response map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("Unmarshal() response error = %v", err)
	}
	rules, ok := response["rules"]
	if !ok {
		t.Fatalf("Marshal() response = %s, missing rules", encoded)
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(rules, &items); err != nil {
		t.Fatalf("Unmarshal() rules error = %v", err)
	}
	for _, field := range []string{"limitType", "unit", "transactionType", "channel", "operator", "value", "source"} {
		if _, ok := items[0][field]; !ok {
			t.Fatalf("Marshal() rule = %s, missing %s", rules, field)
		}
	}
}
