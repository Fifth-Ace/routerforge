//go:build linux

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/apptransaction"
)

func coreTransactionFixture() (appTransactionPreviewRequest, routerForgeReleaseIndex, routerForgeReleaseStatus, []catalogItem) {
	names := []string{"core", "dns", "admin", "monitoring", "network-tools", "nfqws-manager", "antiscan-manager", "port-access-manager", "crowdsec-manager", "dns-router", "profiling"}
	commit := strings.Repeat("a", 40)
	v := "1~dev.r1459." + commit[:12]
	doc := routerForgeReleaseIndex{SchemaVersion: 1, Channel: "dev", Target: "aarch64-3.10", Commit: commit}
	status := routerForgeReleaseStatus{Channel: "dev", Target: doc.Target, Supported: true, Online: true, Source: "remote"}
	var items []catalogItem
	for _, n := range names {
		pkg := "routerforge-" + n
		asset := pkg + "_" + strings.ReplaceAll(v, "~", "-") + "_" + doc.Target + ".ipk"
		rel := catalogRelease{Channel: "dev", Version: v, Package: pkg, Asset: asset, SHA256: strings.Repeat("b", 64), URL: "https://github.com/Fifth-Ace/dns-monitor/releases/download/routerforge-dev/" + asset, CanonicalURL: "https://github.com/Fifth-Ace/routerforge/releases/download/routerforge-dev/" + asset, Architecture: doc.Target, SizeBytes: 123}
		id := n
		if n == "core" {
			id = pkg
		}
		items = append(items, catalogItem{ID: id, Kind: "module", Installed: true, Version: "1~dev.r1458.000000000000", UpdateAvailable: true, Detection: catalogDetection{Packages: []string{pkg}}, Publisher: catalogPublisher{ID: "routerforge"}, Trust: catalogTrust{Status: "official"}, Update: catalogInstallPlan{Method: "routerforge-release", Packages: []string{pkg}}, Actions: catalogActions{Update: true}, Release: rel})
		doc.Components = append(doc.Components, rel)
	}
	return appTransactionPreviewRequest{ExpectedCommit: commit, Packages: []string{"routerforge-core", "routerforge-profiling", "routerforge-dns-router"}}, doc, status, items
}
func TestCoreTransactionPreviewFrozenPlan(t *testing.T) {
	req, doc, status, items := coreTransactionFixture()
	p, e := buildCoreTransactionPreview("test", req, doc, status, items)
	if e != nil {
		t.Fatal(e)
	}
	if p.ExecutionAvailable || p.DownloadBytes != 369 || p.Plan.Packages[0].Name != "routerforge-dns-router" || p.Plan.Packages[1].Name != "routerforge-profiling" || p.Plan.Packages[2].Name != "routerforge-core" {
		t.Fatal(p)
	}
	if h, e := p.Plan.Hash(); e != nil || h != p.PlanSHA256 {
		t.Fatal(h, e)
	}
}
func TestCoreTransactionPreviewRejectsDrift(t *testing.T) {
	for _, mode := range []string{"offline", "cache", "wrong commit", "wrong target", "foreign URL", "duplicate selection", "missing package", "catalog hash", "uninstalled", "denied", "no update", "third-party", "duplicate catalog", "downgrade", "stable installed", "already current"} {
		t.Run(mode, func(t *testing.T) {
			req, doc, status, items := coreTransactionFixture()
			switch mode {
			case "offline":
				status.Online = false
			case "cache":
				status.Source = "cache"
			case "wrong commit":
				req.ExpectedCommit = strings.Repeat("c", 40)
			case "wrong target":
				status.Target = "mips-3.4"
			case "foreign URL":
				doc.Components[0].CanonicalURL = "https://invalid.example/" + doc.Components[0].Asset
			case "duplicate selection":
				req.Packages = append(req.Packages, req.Packages[0])
			case "missing package":
				items = items[1:]
			case "catalog hash":
				items[0].Release.SHA256 = strings.Repeat("c", 64)
			case "uninstalled":
				items[0].Installed = false
			case "denied":
				items[0].Actions.Update = false
			case "no update":
				items[0].UpdateAvailable = false
			case "third-party":
				items[9].Trust.Status = "unverified"
			case "duplicate catalog":
				items = append(items, items[0])
			case "downgrade":
				items[0].Version = "1~dev.r1460.000000000000"
			case "stable installed":
				items[0].Version = "1.0.0"
			case "already current":
				items[0].Version = items[0].Release.Version
			}
			if _, e := buildCoreTransactionPreview("test", req, doc, status, items); e == nil {
				t.Fatal("accepted", mode)
			}
		})
	}
}
func TestCoreTransactionPreviewStrictBody(t *testing.T) {
	for _, raw := range []string{`{"expected_commit":"a","packages":["routerforge-core"],"url":"https://invalid.example"}`, `{"expected_commit":"a","packages":[]} {}`, `{"packages":["` + strings.Repeat("a", 9000) + `"]}`} {
		r := httptest.NewRequest(http.MethodPost, "/api/apps/transactions/preview", strings.NewReader(raw))
		if _, e := decodeTransactionPreview(httptest.NewRecorder(), r); e == nil {
			t.Fatal("accepted unsafe request")
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/apps/transactions/preview", strings.NewReader(`{"expected_commit":"a","packages":["routerforge-core"]}`))
	if _, e := decodeTransactionPreview(httptest.NewRecorder(), r); e != nil {
		t.Fatal(e)
	}
}
func TestCoreTransactionStartNotExposed(t *testing.T) {
	mux := http.NewServeMux()
	registerCoreAppTransactionHandlers(mux, &authManager{})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/apps/transactions", strings.NewReader(`{}`)))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestCoreTransactionRootRequired(t *testing.T) {
	mux := http.NewServeMux()
	registerCoreAppTransactionHandlers(mux, &authManager{config: securityConfig{AuthRequired: true}})
	for _, path := range []string{"/api/apps/transactions", "/api/apps/transactions/preview"} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusForbidden {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}

func TestCoreTransactionLegacyAdmission(t *testing.T) {
	d := t.TempDir()
	if err := os.Chmod(d, 0700); err != nil {
		t.Fatal(err)
	}
	c, err := apptransaction.OpenCoordinator(d)
	if err != nil {
		t.Fatal(err)
	}
	state := coreTransactionState{enabled: true, coordinator: c}
	release, err := state.acquire()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = state.acquire(); !errors.Is(err, apptransaction.ErrBusy) || coreMutationHTTPStatus(err) != http.StatusConflict {
		release()
		t.Fatal(err)
	}
	release()
	release, err = state.acquire()
	if err != nil {
		t.Fatal(err)
	}
	release()
	state.err = errors.New("unsafe root")
	if _, err = state.acquire(); err == nil || coreMutationHTTPStatus(err) != http.StatusServiceUnavailable {
		t.Fatal(err)
	}
	state.enabled = false
	release, err = state.acquire()
	if err != nil {
		t.Fatal(err)
	}
	release()
}
