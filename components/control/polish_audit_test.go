package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDecodeMutationJSONRejectsTrailingValue(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"value":"one"}{"value":"two"}`))
	var target struct {
		Value string `json:"value"`
	}
	if err := decodeMutationJSON(rec, req, &target); err == nil {
		t.Fatal("expected trailing JSON value rejection")
	}
}

func TestAdminFileMtimeJSONIsBrowserSafeString(t *testing.T) {
	value := int64(1700000000123456789)

	entryJSON, err := json.Marshal(adminFileEntry{ModifiedAtNS: value})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(entryJSON, []byte(`"mtime_ns":"1700000000123456789"`)) {
		t.Fatalf("entry mtime_ns is not encoded as string: %s", entryJSON)
	}

	requestJSON, err := json.Marshal(adminFileWriteRequest{ExpectedMtimeNS: &value})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(requestJSON, []byte(`"expected_mtime_ns":"1700000000123456789"`)) {
		t.Fatalf("request expected_mtime_ns is not encoded as string: %s", requestJSON)
	}

	var decoded adminFileWriteRequest
	if err := json.Unmarshal(requestJSON, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ExpectedMtimeNS == nil || *decoded.ExpectedMtimeNS != value {
		t.Fatalf("decoded mtime=%v", decoded.ExpectedMtimeNS)
	}
}

func TestRunAdminWatchdogServiceStartTimesOut(t *testing.T) {
	script := t.TempDir() + "/slow-start"
	if err := osWriteExecutable(script, "#!/bin/sh\nexec sleep 2\n"); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	attempt := runAdminWatchdogServiceStart(script, started, 50*time.Millisecond)
	elapsed := time.Since(started)
	if attempt.OK {
		t.Fatal("timed out watchdog start reported success")
	}
	if !strings.Contains(attempt.Output, "timed out") {
		t.Fatalf("missing timeout marker: %q", attempt.Output)
	}
	if elapsed > time.Second {
		t.Fatalf("watchdog timeout took too long: %s", elapsed)
	}
}

func osWriteExecutable(path, content string) error {
	return os.WriteFile(path, []byte(content), 0755)
}
