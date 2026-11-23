package reservation

import "testing"

func TestStatusAndEntryTypeValid(t *testing.T) {
	for _, status := range []Status{StatusProcessing, StatusReserved, StatusReleased, StatusRejected, StatusFailed} {
		if !status.Valid() {
			t.Errorf("%q must be valid", status)
		}
	}
	for _, status := range []Status{"PENDING", "COMMITTED", "EXPIRED"} {
		if status.Valid() {
			t.Errorf("%q must be invalid", status)
		}
	}
	if !EntryReserve.Valid() || !EntryRelease.Valid() || EntryType("ADJUST").Valid() {
		t.Error("entry type validation is wrong")
	}
}
