package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestAdminFileListExposesExactMtimeNSForGuardedMutations(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/list?path="+root, nil)
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	adminFileAccessOnly(false, handleAdminFileList)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Entries []adminFileEntry `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Entries) != 1 {
		t.Fatalf("entries=%d", len(body.Entries))
	}
	if body.Entries[0].ModifiedAtNS != info.ModTime().UnixNano() {
		t.Fatalf("mtime_ns=%d want=%d", body.Entries[0].ModifiedAtNS, info.ModTime().UnixNano())
	}
}

func TestAdminFileReadExposesExactMtimeNSForEditorPrecondition(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config")
	if err := os.WriteFile(target, []byte("alpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/read?path="+target, nil)
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	adminFileAccessOnly(false, handleAdminFileRead)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var body adminFileReadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ModifiedAtNS != info.ModTime().UnixNano() {
		t.Fatalf("mtime_ns=%d want=%d", body.ModifiedAtNS, info.ModTime().UnixNano())
	}
}
