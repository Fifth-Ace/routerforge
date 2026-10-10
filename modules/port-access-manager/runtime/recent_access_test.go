package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecentEntriesParse(t *testing.T) {
	p := filepath.Join(t.TempDir(), "recent")
	if err := os.WriteFile(p, []byte("src=192.0.2.4 ttl: 64 last_seen: 123 oldest_pkt: 1\nsrc=2001:db8::1 ttl: 64\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readRecentClients(p, 120, time.Now())
	if err != nil || len(got) != 2 || got[0].IP != "192.0.2.4" {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if got[0].TTLKnown {
		t.Fatal("jiffies are not wall-clock duration")
	}
}
func TestRevokeNeedsCore(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/recent-revoke", strings.NewReader(`{"ip":"192.0.2.4","confirm":"REVOKE"}`))
	recentRevokeHandler(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
