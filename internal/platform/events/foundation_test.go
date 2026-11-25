package events

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

func TestFoundationEventIdentityAndProjection(t *testing.T) {
	r := NewRing(2)
	e := Event{Component: "core", Severity: Info, Type: "core.ready", Message: "SECRET_RAW", ObjectID: "SECRET_OBJECT", Context: map[string]any{"token": "SECRET_TOKEN"}}
	r.Add(e)
	r.Add(e)
	a := r.Snapshot(0)
	if a[0].ID == a[1].ID || !regexp.MustCompile(`^[0-9a-f-]{36}$`).MatchString(a[0].ID) {
		t.Fatal("event identity invalid")
	}
	id, err := NewEventID()
	if err != nil {
		t.Fatal(err)
	}
	wire, ok := Normalize(a[0], id)
	if !ok || wire.ID != r.Snapshot(0)[0].ID || wire.MessageCode != "CORE_READY" {
		t.Fatal(wire)
	}
	b, _ := json.Marshal(wire)
	if strings.Contains(string(b), "SECRET") || !strings.Contains(string(b), `"deviceId":null`) {
		t.Fatal("projection leaked or guessed data")
	}
	e.Type = "arbitrary.SECRET"
	r.Add(e)
	if _, ok := Normalize(r.Snapshot(0)[1], id); ok {
		t.Fatal("unknown event exported")
	}
}
