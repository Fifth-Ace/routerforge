package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// This gate must remain false until actual generated kernel rules are proven
// to enforce the strict model for all packet traces. A green CI does not
// authorize deployment.
func TestK4OStrictSequenceFailClosed(t *testing.T) {
	p, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil || p.Hooked || p.Applied || p.StrictSequenceVerified {
		t.Fatalf("unexpectedly enabled plan: %+v %v", p, err)
	}
	h := routes(t.TempDir())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/knock-preview?first=41001&second=41002&third=41003&target=2222&window=30&ttl=120", nil))
	if w.Code != 200 {
		t.Fatalf("status %d", w.Code)
	}
	var got struct {
		Applied bool `json:"applied"`
		Hooked  bool `json:"hooked"`
		Strict  bool `json:"strict_sequence_verified"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil || got.Applied || got.Hooked || got.Strict {
		t.Fatalf("preview must remain blocked: %+v %v", got, err)
	}
}

func TestK4OStrictModelRejectsInterleaving(t *testing.T) {
	paths := [][]int{
		{41001, 41003, 41002, 41003},
		{41001, 41002, 41002, 41003},
		{41001, 41002, 41001, 41003},
		{41001, 41002, 41003, 41001, 41003},
	}
	for _, ports := range paths {
		m := newStrictKnockModel()
		for i, p := range ports {
			m.packet("198.51.100.10", p, i, 30, 120, 2222)
		}
		if m.packet("198.51.100.10", 2222, len(ports), 30, 120, 2222) {
			t.Fatalf("invalid trace authorized: %v", ports)
		}
	}
}
