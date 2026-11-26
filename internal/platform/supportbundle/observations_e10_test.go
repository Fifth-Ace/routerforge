package supportbundle

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestE10ObservationPrivacy(t *testing.T) {
	raw := []byte(`{"ok":true,"probe_observations":{"samples":2,"successful":1,"failed":1,"target":"secret.example"},"storage_observations":{"tracked_disks":1,"counter_baselines":1,"serial":"secret-serial"}}`)
	n := ProjectHealth("network-tools", raw)
	if n.ProbeObservations == nil || n.StorageObservations != nil {
		t.Fatal(n)
	}
	s := ProjectHealth("monitoring", raw)
	if s.StorageObservations == nil || s.ProbeObservations != nil {
		t.Fatal(s)
	}
	b, _ := json.Marshal([]Health{n, s})
	if strings.Contains(string(b), "secret") {
		t.Fatal(string(b))
	}
	for _, bad := range []string{`{"samples":65,"successful":65,"failed":0}`, `{"samples":2,"successful":2,"failed":1}`, `{"samples":0,"failed":0}`} {
		if ProjectHealth("network-tools", []byte(`{"ok":true,"probe_observations":`+bad+`}`)).ProbeObservations != nil {
			t.Fatal(bad)
		}
	}
}
