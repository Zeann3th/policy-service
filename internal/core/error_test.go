package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestAppErrorWrapsCauseWithoutExposingIt(t *testing.T) {
	cause := errors.New("database connection details")
	err := ErrNotFound.WithCause(cause)

	if !errors.Is(err, ErrNotFound) {
		t.Fatal("errors.Is() did not match ErrNotFound")
	}
	if !errors.Is(err, cause) {
		t.Fatal("errors.Is() did not unwrap the cause")
	}

	encoded, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("Marshal() error = %v", marshalErr)
	}
	if strings.Contains(string(encoded), cause.Error()) || string(encoded) != `{"code":"NOT_FOUND","message":"not found"}` {
		t.Fatalf("Marshal() = %s", encoded)
	}
}
