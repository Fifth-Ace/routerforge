package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func d1UseDetectorStubs(t *testing.T, configSHA string) (string, *[]string) {
	t.Helper()
	oldPaths := dpiDetectorCandidatePaths
	oldRunner := dpiDetectorCommandRunner
	oldStatus := dpiDetectorStatusReader
	oldRunning := dpiDetectorNFQWS2Running
	t.Cleanup(func() {
		dpiDetectorCandidatePaths = oldPaths
		dpiDetectorCommandRunner = oldRunner
		dpiDetectorStatusReader = oldStatus
		dpiDetectorNFQWS2Running = oldRunning
		atomic.StoreInt32(&dpiDetectorRunActive, 0)
	})

	dir := t.TempDir()
	path := filepath.Join(dir, "dpi-detector")
	if err := os.WriteFile(path, []byte("stub"), 0700); err != nil {
		t.Fatal(err)
	}
	dpiDetectorCandidatePaths = []string{path}
	dpiDetectorStatusReader = func() managerStatus { return managerStatus{ConfigSHA256: configSHA} }
	dpiDetectorNFQWS2Running = func() bool { return true }
	captured := []string{}
	dpiDetectorCommandRunner = func(_ context.Context, _ int, program string, args ...string) ([]byte, error) {
		if program != path {
			t.Fatalf("program=%q want=%q", program, path)
		}
		captured = append([]string{}, args...)
		report := ""
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-o" {
				report = args[i+1]
				break
			}
		}
		if report == "" {
			t.Fatal("-o report path missing")
		}
		if err := os.WriteFile(report, []byte("DPI Detector report\nexample.com OK\n"), 0600); err != nil {
			t.Fatal(err)
		}
		return []byte("stub stdout"), nil
	}
	atomic.StoreInt32(&dpiDetectorRunActive, 0)
	return path, &captured
}

func TestD1BuildDetectorRunArgsFixedContract(t *testing.T) {
	got := buildDPIDetectorRunArgs("example.com", 3, "/tmp/report.txt")
	want := []string{"-t", "2", "-d", "example.com", "-c", "3", "-o", "/tmp/report.txt", "--batch"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("args=%q want=%q", got, want)
	}
	joined := strings.Join(got, " ")
	for _, forbidden := range []string{"-p ", "--proxy", "-v", "--verbose", "-t 1", "-t 3", "-t 4", "-t 5"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("forbidden argument leaked: %q in %q", forbidden, joined)
		}
	}
}

func TestD1ValidateDetectorRunRequest(t *testing.T) {
	sha := strings.Repeat("a", 64)
	domain, concurrency, err := validateDPIDetectorRunRequest(dpiDetectorRunRequest{
		Test: "domains", Domain: "example.com", ExpectedConfigSHA256: sha, Confirm: dpiDetectorRunConfirm,
	})
	if err != nil {
		t.Fatal(err)
	}
	if domain != "example.com" || concurrency != 2 {
		t.Fatalf("domain=%q concurrency=%d", domain, concurrency)
	}

	bad := []dpiDetectorRunRequest{
		{Test: "tcp16", Domain: "example.com", ExpectedConfigSHA256: sha, Confirm: dpiDetectorRunConfirm},
		{Test: "domains", Domain: "localhost", ExpectedConfigSHA256: sha, Confirm: dpiDetectorRunConfirm},
		{Test: "domains", Domain: "example.com", Concurrency: 6, ExpectedConfigSHA256: sha, Confirm: dpiDetectorRunConfirm},
		{Test: "domains", Domain: "example.com", ExpectedConfigSHA256: "bad", Confirm: dpiDetectorRunConfirm},
		{Test: "domains", Domain: "example.com", ExpectedConfigSHA256: sha, Confirm: "wrong"},
	}
	for i, req := range bad {
		if _, _, err := validateDPIDetectorRunRequest(req); err == nil {
			t.Fatalf("bad request[%d] accepted: %+v", i, req)
		}
	}
}

func TestD1DetectorRunProducesBoundedEnvelope(t *testing.T) {
	sha := strings.Repeat("a", 64)
	path, captured := d1UseDetectorStubs(t, sha)

	body, _ := json.Marshal(dpiDetectorRunRequest{
		Test: "domains", Domain: "example.com", Concurrency: 3,
		ExpectedConfigSHA256: sha, Confirm: dpiDetectorRunConfirm,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/dpi-detector/run", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleDPIDetectorRun(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var resp dpiDetectorRunResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !resp.OK || resp.Test != "domains" || resp.UpstreamTest != "2" || resp.Domain != "example.com" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Path != path || !resp.Batch || resp.Concurrency != 3 {
		t.Fatalf("execution metadata mismatch: %+v", resp)
	}
	if !strings.Contains(resp.Report, "example.com OK") || resp.ReportCut || resp.ReportBytes <= 0 {
		t.Fatalf("report mismatch: %+v", resp)
	}
	if !resp.NFQWS2Running || resp.RawProviderTruth || resp.PersistentMutation || resp.ManagedByRouterForge {
		t.Fatalf("safety/environment flags mismatch: %+v", resp)
	}
	if !resp.ProductionConfigUnchanged || resp.ProductionConfigSHA256 != sha {
		t.Fatalf("production config proof mismatch: %+v", resp)
	}
	wantPrefix := []string{"-t", "2", "-d", "example.com", "-c", "3", "-o"}
	if len(*captured) < len(wantPrefix)+2 || !reflect.DeepEqual((*captured)[:len(wantPrefix)], wantPrefix) || (*captured)[len(*captured)-1] != "--batch" {
		t.Fatalf("captured args=%q", *captured)
	}
}

func TestD1DetectorRunAbsentIsFailClosed(t *testing.T) {
	oldPaths := dpiDetectorCandidatePaths
	oldStatus := dpiDetectorStatusReader
	t.Cleanup(func() {
		dpiDetectorCandidatePaths = oldPaths
		dpiDetectorStatusReader = oldStatus
	})
	sha := strings.Repeat("a", 64)
	dpiDetectorCandidatePaths = []string{filepath.Join(t.TempDir(), "dpi-detector")}
	dpiDetectorStatusReader = func() managerStatus { return managerStatus{ConfigSHA256: sha} }

	body, _ := json.Marshal(dpiDetectorRunRequest{
		Test: "domains", Domain: "example.com", ExpectedConfigSHA256: sha, Confirm: dpiDetectorRunConfirm,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/dpi-detector/run", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handleDPIDetectorRun(rr, req)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), "not installed") {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestD1DetectorRunRouteIsPostOnly(t *testing.T) {
	mux := http.NewServeMux()
	registerDPIDetectorRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/dpi-detector/run", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET run status=%d body=%s", rr.Code, rr.Body.String())
	}
}
