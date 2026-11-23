package reservation

import "testing"

func TestStatusAndCommitSourceValid(t *testing.T) {
	for _, status := range []Status{StatusPending, StatusReserved, StatusCommitted, StatusReleased, StatusExpired, StatusRejected} {
		if !status.Valid() {
			t.Errorf("%q must be valid", status)
		}
	}
	if Status("CANCELLED").Valid() {
		t.Error("CANCELLED must be invalid")
	}
	if !CommitSourceEvent.Valid() || !CommitSourceAuto.Valid() || CommitSource("MANUAL").Valid() {
		t.Error("commit source validation is wrong")
	}
}
