package reservation

import (
	"time"

	"github.com/zeann3th/policy-service/internal/core/policy"
)

var lifetimeStart = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)

// windowStarts is where each accumulating window begins (UTC calendar).
// TRANSACTION is not here: a policy over it looks at the transaction alone
// and keeps no usage.
var windowStarts = map[policy.WindowType]func(utc time.Time) time.Time{
	policy.WindowDaily: func(utc time.Time) time.Time {
		return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	},
	policy.WindowMonthly: func(utc time.Time) time.Time {
		return time.Date(utc.Year(), utc.Month(), 1, 0, 0, 0, 0, time.UTC)
	},
	policy.WindowLifetime: func(time.Time) time.Time { return lifetimeStart },
}

// accumulates reports whether usage adds up over the window.
func accumulates(window policy.WindowType) bool {
	_, ok := windowStarts[window]
	return ok
}

// windowStart is the start of the accumulating window that holds at.
func windowStart(window policy.WindowType, at time.Time) time.Time {
	return windowStarts[window](at.UTC())
}
