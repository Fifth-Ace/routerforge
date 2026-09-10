package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func authorizeAdminFileRequest(req *http.Request) {
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
}

func withReadAdminFileRoot(t *testing.T, root string) {
	t.Helper()
	oldRoots := adminFileAllowedRoots
	oldEval := adminFileEvalSymlinks
	adminFileAllowedRoots = []string{root}
	adminFileEvalSymlinks = filepath.EvalSymlinks
	t.Cleanup(func() {
		adminFileAllowedRoots = oldRoots
		adminFileEvalSymlinks = oldEval
	})
}

func TestAdminFileAccessRequiresCoreAuthorizationMarker(t *testing.T) {
	handler := adminFileAccessOnly(false, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/list?path=%2Ftmp", nil)
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAdminFileListReturnsBoundedMetadata(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	if err := os.Mkdir(filepath.Join(root, "adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("hello"), 0o640); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/list?path="+url.QueryEscape(root), nil)
	authorizeAdminFileRequest(req)
	handle := adminFileAccessOnly(false, handleAdminFileList)
	handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Path       string           `json:"path"`
		EntryCount int              `json:"entry_count"`
		Entries    []adminFileEntry `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Path != root || payload.EntryCount != 2 || len(payload.Entries) != 2 {
		t.Fatalf("payload=%+v", payload)
	}
	if payload.Entries[0].Name != "adir" || payload.Entries[0].Kind != "directory" {
		t.Fatalf("first=%+v", payload.Entries[0])
	}
	if payload.Entries[1].Name != "b.txt" || payload.Entries[1].Kind != "file" {
		t.Fatalf("second=%+v", payload.Entries[1])
	}
}

func TestAdminFileReadReturnsUTF8Text(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	file := filepath.Join(root, "config.txt")
	if err := os.WriteFile(file, []byte("alpha\nβeta\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/read?path="+url.QueryEscape(file), nil)
	authorizeAdminFileRequest(req)
	adminFileAccessOnly(false, handleAdminFileRead)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload adminFileReadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Encoding != "utf-8" || payload.Content != "alpha\nβeta\n" {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestAdminFileReadRejectsBinaryAndOversizedContent(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	binaryFile := filepath.Join(root, "binary.bin")
	if err := os.WriteFile(binaryFile, []byte{'a', 0, 'b'}, 0o600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/read?path="+url.QueryEscape(binaryFile), nil)
	authorizeAdminFileRequest(req)
	adminFileAccessOnly(false, handleAdminFileRead)(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("binary status=%d body=%s", rec.Code, rec.Body.String())
	}

	largeFile := filepath.Join(root, "large.txt")
	if err := os.WriteFile(largeFile, []byte(strings.Repeat("x", int(adminFileReadLimit)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/files/read?path="+url.QueryEscape(largeFile), nil)
	authorizeAdminFileRequest(req)
	adminFileAccessOnly(false, handleAdminFileRead)(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminFileDownloadSupportsGetAndHead(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	file := filepath.Join(root, "archive.bin")
	content := []byte{1, 2, 3, 4, 5}
	if err := os.WriteFile(file, content, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/v1/files/download?path="+url.QueryEscape(file), nil)
		authorizeAdminFileRequest(req)
		adminFileAccessOnly(true, handleAdminFileDownload)(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", method, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "archive.bin") {
			t.Fatalf("%s disposition=%q", method, got)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
			t.Fatalf("%s nosniff=%q", method, got)
		}
		if method == http.MethodGet && string(rec.Body.Bytes()) != string(content) {
			t.Fatalf("GET body=%v", rec.Body.Bytes())
		}
		if method == http.MethodHead && rec.Body.Len() != 0 {
			t.Fatalf("HEAD body len=%d", rec.Body.Len())
		}
	}
}

func TestAdminFileListRejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	unsafe := filepath.Join(root, "safe") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "escape"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/files/list?path="+url.QueryEscape(unsafe), nil)
	authorizeAdminFileRequest(req)
	adminFileAccessOnly(false, handleAdminFileList)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
