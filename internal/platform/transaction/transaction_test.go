package transaction

import "testing"

func TestTransitions(t *testing.T) {
	m := &Manifest{State: Precheck}
	for _, state := range []State{Snapshot, Validated, Applied, Verified, Committed} {
		if err := Advance(m, state); err != nil {
			t.Fatalf("transition to %s failed: %v", state, err)
		}
	}
	if err := Advance(m, Failed); err == nil {
		t.Fatal("terminal state accepted transition")
	}
}
