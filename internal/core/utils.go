package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func Join(args ...any) string {
	var b strings.Builder
	for i, arg := range args {
		if i > 0 {
			b.WriteString(":")
		}
		fmt.Fprint(&b, arg)
	}
	return b.String()
}

func Encode[T any](value *T) (string, error) {
	valueByte, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(valueByte), nil
}

func Decode[T any](value string) (*T, error) {
	var result T
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// Code normalizes a catalog code (unit, policy type) to upper case and
// reports whether it is well formed: a letter, then letters, digits or
// underscores ("TXN_FEE", not "COIN MAX").
func Code(value string) (string, bool) {
	code := strings.ToUpper(strings.TrimSpace(value))
	return code, codePattern.MatchString(code)
}
