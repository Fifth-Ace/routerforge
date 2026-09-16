package transaction

import (
	"errors"
	"testing"
)

func TestTransitions(t *testing.T) {
	m := New("test", "admin")
	for _, state := range []State{Snapshot, Validated, Applied, Verified, Committed} {
		if err := Advance(&m, state); err != nil {
			t.Fatalf("transition to %s failed: %v", state, err)
		}
	}
	if err := Advance(&m, Failed); err == nil {
		t.Fatal("terminal state accepted transition")
	}
	if len(m.Evidence) != 6 {
		t.Fatalf("evidence entries = %d, want 6", len(m.Evidence))
	}
	if m.Evidence[0].Stage != string(Precheck) || m.Evidence[len(m.Evidence)-1].Stage != string(Committed) {
		t.Fatalf("unexpected evidence lifecycle: %+v", m.Evidence)
	}
}

func TestWrapExtractPreservesCauseAndEvidence(t *testing.T) {
	cause := errors.New("probe failed")
	m := New("dns-test", "dns", "DoH")
	if err := Advance(&m, Snapshot); err != nil {
		t.Fatal(err)
	}
	RecordFailure(&m, "runtime-probe", cause, map[string]string{"probe": "unix-health"})
	if err := Advance(&m, Failed); err != nil {
		t.Fatal(err)
	}

	wrapped := Wrap(cause, m)
	if !errors.Is(wrapped, cause) {
		t.Fatal("wrapped transaction error lost original cause")
	}
	got, ok := Extract(wrapped)
	if !ok {
		t.Fatal("transaction evidence not extractable")
	}
	if got.ID != m.ID || got.State != Failed || got.Error != cause.Error() {
		t.Fatalf("unexpected extracted manifest: %+v", got)
	}
	if len(got.Evidence) < 3 {
		t.Fatalf("expected evidence trail, got %+v", got.Evidence)
	}
}

func TestEvidenceIsBoundedAndCloned(t *testing.T) {
	m := New("bounded", "dns")
	for i := 0; i < MaxEvidenceEntries+20; i++ {
		Record(&m, "step", EvidenceInfo, "event", map[string]string{"k": "v"})
	}
	if len(m.Evidence) != MaxEvidenceEntries {
		t.Fatalf("evidence entries = %d, want %d", len(m.Evidence), MaxEvidenceEntries)
	}
	if m.Evidence[0].Stage != string(Precheck) {
		t.Fatal("bounded evidence must retain transaction origin")
	}
	cloned := Clone(m)
	cloned.Evidence[len(cloned.Evidence)-1].Details["k"] = "changed"
	if m.Evidence[len(m.Evidence)-1].Details["k"] != "v" {
		t.Fatal("Clone leaked mutable evidence details")
	}
}
