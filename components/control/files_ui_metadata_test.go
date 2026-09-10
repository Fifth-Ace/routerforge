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

func TestAdminFileListTreeMetadataMarksExpandableDirectories(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	emptyDir := filepath.Join(root, "empty")
	filesOnlyDir := filepath.Join(root, "files-only")
	nestedDir := filepath.Join(root, "nested")
	for _, dir := range []string{emptyDir, filesOnlyDir, nestedDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(filesOnlyDir, "config"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(nestedDir, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/list?path="+root+"&tree_meta=1", nil)
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

	got := make(map[string]*bool, len(body.Entries))
	for _, entry := range body.Entries {
		got[entry.Name] = entry.HasChildDirectories
	}

	assertTreeMeta := func(name string, want bool) {
		t.Helper()
		value, ok := got[name]
		if !ok {
			t.Fatalf("missing entry %q", name)
		}
		if value == nil {
			t.Fatalf("entry %q has no tree metadata", name)
		}
		if *value != want {
			t.Fatalf("entry %q has_child_directories=%v want=%v", name, *value, want)
		}
	}

	assertTreeMeta("empty", false)
	assertTreeMeta("files-only", false)
	assertTreeMeta("nested", true)
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
