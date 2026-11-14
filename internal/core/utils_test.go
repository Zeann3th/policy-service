package core

import "testing"

type testValue struct {
	Name string `json:"name"`
}

func TestJoin(t *testing.T) {
	if got := Join("service", "PERSONAL", "USD", "TIER_1"); got != "service:PERSONAL:USD:TIER_1" {
		t.Fatalf("Join() = %q", got)
	}
}

func TestEncodeDecode(t *testing.T) {
	want := testValue{Name: "limit"}
	encoded, err := Encode(&want)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	got, err := Decode[testValue](encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if *got != want {
		t.Fatalf("Decode() = %#v, want %#v", *got, want)
	}
}
