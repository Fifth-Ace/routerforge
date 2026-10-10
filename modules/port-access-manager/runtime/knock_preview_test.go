package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestKnockPreviewContract(t *testing.T) {
	h := routes(t.TempDir())
	ok := "/v1/knock-preview?first=41001&second=41002&third=41003&target=2222&window=30&ttl=120"
	for _, tc := range []struct {
		method, path string
		want         int
	}{{"GET", ok, 200}, {"HEAD", ok, 200}, {"POST", ok, 405}, {"GET", ok + "&evil=1", 400}, {"GET", ok + "&first=9", 400}, {"GET", "/v1/knock-preview?first=22", 400}} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s %s: %d", tc.method, tc.path, w.Code)
		}
		if tc.want == 200 && tc.method == "GET" {
			var p KnockRules
			if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Applied || p.Hooked || len(p.Commands) != 6 {
				t.Fatalf("unsafe payload %+v err %v", p, err)
			}
		}
	}
}
