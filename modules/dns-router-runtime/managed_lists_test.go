package dnsrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestP5ManagedListParsingAndPreservation(t *testing.T) {
	cfg := EmptyConfig()
	cfg.Upstreams = []Upstream{{ID: "primary", Transport: "udp", Address: "9.9.9.9:53"}}
	cfg.Listener = &ListenerConfig{Mode: "loopback", Listen: "127.0.0.1:55353"}
	cfg.Rules = []Rule{{ID: "manual", Domain: "manual.example.org", Match: "suffix", Upstream: "primary", Priority: 25}}
	req := ManagedListRequest{List: DomainList{ID: "ytlist", Name: "YouTube", Kind: "file", Upstream: "primary", Match: "suffix", Priority: 100}, Text: "# comment\nyoutube.com\n||googlevideo.com^\n0.0.0.0 ytimg.com\nyoutube.com\n@@||exception.example.com^\nwrong:bad\n"}
	preview, err := PreviewManagedList(cfg, req)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Added != 3 || preview.Duplicates != 1 || preview.Invalid != 1 || preview.Ignored != 2 {
		t.Fatalf("incorrect counts %+v", preview)
	}
	if preview.TotalRules != 4 || len(preview.Config.Lists) != 1 || preview.Config.Listener == nil {
		t.Fatalf("existing policy lost: %+v", preview)
	}
	for _, r := range preview.Config.Rules {
		if r.ID != "manual" && r.ListID != "ytlist" {
			t.Fatalf("list identity lost: %+v", r)
		}
	}
	if _, err := Compile(preview.Config); err != nil {
		t.Fatal(err)
	}
	req.Text = "youtube.com\nnewexample.org\n"
	updated, err := PreviewManagedList(preview.Config, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Config.Rules) != 3 || updated.Config.Rules[0].ID != "manual" {
		t.Fatalf("replace/list-scoped update affected other rules %+v", updated)
	}
}

func TestP5ManagedListInvalidReferences(t *testing.T) {
	cfg := EmptyConfig()
	cfg.Upstreams = []Upstream{{ID: "p", Transport: "udp", Address: "9.9.9.9:53"}}
	cfg.Rules = []Rule{{ID: "x", Domain: "x.example.com", Match: "suffix", Upstream: "p", ListID: "ghost", Priority: 100}}
	if _, err := Compile(cfg); err == nil {
		t.Fatal("orphaned list reference accepted")
	}
}

func TestP5RejectLocalSourceURL(t *testing.T) {
	urls := []string{"http://example.org/list.txt", "https://127.0.0.1/list", "https://localhost/list", "https://192.168.10.1/list", "https://example.org:8443/x", "https://u:p@example.org/x", "https://example.org/x#frag", "https://example.local/list"}
	for _, v := range urls {
		if _, err := validatePublicListURL(v); err == nil {
			t.Errorf("accepted %s", v)
		}
	}
	if _, err := validatePublicListURL("https://example.org/list.txt"); err != nil {
		t.Fatal(err)
	}
	if publicListIP([]byte{127, 0, 0, 1}) {
		t.Fatal("loopback accepted")
	}
	if _, err := FetchManagedListText(context.Background(), "http://127.0.0.1/"); err == nil {
		t.Fatal("private fetch accepted")
	}
}

func TestP5LegacyImportPreservesLists(t *testing.T) {
	cfg := EmptyConfig()
	cfg.Upstreams = []Upstream{{ID: "p", Transport: "udp", Address: "9.9.9.9:53"}}
	cfg.Lists = []DomainList{{ID: "list", Name: "original", Kind: "paste", Upstream: "p", Match: "suffix", Priority: 100}}
	cfg.Rules = []Rule{{ID: "imported", Domain: "old.example.com", Match: "suffix", Upstream: "p", Priority: 100, ListID: "list"}}
	preview, err := PreviewImport(cfg, ImportRequest{Text: "new.example.com", Upstream: "p", Match: "suffix", Priority: 100, Mode: "append"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Config.Lists) != 1 || !strings.Contains(preview.Config.Lists[0].Name, "original") {
		t.Fatal("legacy import discarded list metadata")
	}
}

func TestP5APIListPreviewDoesNotPersistOrActivateDNS(t *testing.T) {
	store := NewConfigStore(filepath.Join(t.TempDir(), "cfg.json"))
	cfg, rev, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.Upstreams = []Upstream{{ID: "p", Transport: "udp", Address: "9.9.9.9:53"}}
	rev, err = store.Save(cfg, rev)
	if err != nil {
		t.Fatal(err)
	}
	request := ManagedListRequest{List: DomainList{ID: "sample", Name: "Sample", Kind: "paste", Upstream: "p", Match: "suffix", Priority: 100}, Text: "youtube.com\ndiscord.com"}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/v1/lists/preview", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-RouterForge-Action", "lists-preview")
	w := httptest.NewRecorder()
	NewAPI(store, "").Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"saved":false`) {
		t.Fatalf("preview rejected: HTTP %d %s", w.Code, w.Body.String())
	}
	after, newRev, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if rev != newRev || len(after.Lists) != 0 || len(after.Rules) != 0 {
		t.Fatal("preview unexpectedly persisted policy")
	}
}
