package main

import (
	"strings"
	"testing"
)

func TestStrategyImportParserBatchParity(t *testing.T) {
	src := `@echo off
set "BIN=%~dp0bin\"
set "LISTS=%~dp0lists\"
set "GameFilter=1024-65535"
start "" "%BIN%winws.exe" --wf-tcp=80,443 --wf-udp=443 ^
 --filter-tcp=443 --hostlist="%LISTS%list-general.txt" --dpi-desync=fake --dpi-desync-fake-tls="%BIN%tls_clienthello_www_google_com.bin" ^
 --new ^
 --filter-udp=443 --dpi-desync=fake --dpi-desync-repeats=6
`
	got, err := v2StrategyImportParse(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "batch" || !got.Ready || len(got.Commands) != 1 || len(got.Profiles) != 2 {
		t.Fatalf("unexpected parse: %+v", got)
	}
	if got.TCP != "80,443" || got.UDP != "443" {
		t.Fatalf("wf tcp=%q udp=%q", got.TCP, got.UDP)
	}
	joined := strings.Join(got.Profiles, " ")
	for _, want := range []string{
		"/opt/etc/nfqws2/lists/list-general.txt",
		"/opt/etc/nfqws2/blobs/tls_clienthello.bin",
		"--filter-udp=443",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
	if len(got.Deps) < 2 {
		t.Fatalf("deps=%+v", got.Deps)
	}
}

func TestStrategyImportParserRejectsAmbiguousMultipleProcesses(t *testing.T) {
	got, err := v2StrategyImportParse("winws.exe --filter-tcp=443 --dpi-desync=fake\nwinws.exe --filter-tcp=80 --dpi-desync=fake")
	if err != nil {
		t.Fatal(err)
	}
	if got.Ready || len(got.Commands) != 2 {
		t.Fatalf("ambiguous source accepted: %+v", got)
	}
	found := false
	for _, w := range got.Warnings {
		if w.Level == "bad" && strings.Contains(w.Text, "Multiple") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing ambiguity warning: %+v", got.Warnings)
	}
}

func TestStrategyImportParserUnresolvedVariableFailsClosed(t *testing.T) {
	got, err := v2StrategyImportParse(`winws.exe --filter-tcp=443 --hostlist=%MISSING% --dpi-desync=fake`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Ready {
		t.Fatalf("unresolved variable accepted: %+v", got)
	}
}

func TestStrategyImportParserLogFormat1(t *testing.T) {
	src := `Config: general ALT (Type: TCP)
  Target: youtube.com (Google)
  HTTP: code=200 size=12.5 KB status=OK
  TLS1.2: code=0 size=0 bytes status=FAIL
`
	got, err := v2StrategyImportParse(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "log" || got.Ready || got.Log.Format != "format1" || got.Log.Strategies != 1 ||
		got.Log.Targets != 1 || got.Log.Tests != 2 || got.Log.Passed != 1 || got.Log.Failed != 1 {
		t.Fatalf("unexpected log parse: %+v", got)
	}
}

func TestStrategyImportParserLogFormat2(t *testing.T) {
	src := `[1/2] general ALT
=== youtube.com [Google] ===
[youtube.com][HTTP] code=200 size=12000 bytes status=OK
[youtube.com][TLS1.3] code=0 size=0 bytes status=FAIL
`
	got, err := v2StrategyImportParse(src)
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "log" || got.Log.Format != "format2" || got.Log.Strategies != 1 ||
		got.Log.Targets != 1 || got.Log.Tests != 2 || got.Log.Passed != 1 || got.Log.Failed != 1 {
		t.Fatalf("unexpected log parse: %+v", got)
	}
}

func TestStrategyImportParserProductionMutationAlwaysFalse(t *testing.T) {
	got, err := v2StrategyImportParse(`nfqws --filter-tcp=443 --dpi-desync=fake`)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProductionMutation {
		t.Fatal("parser must not mutate production")
	}
}
