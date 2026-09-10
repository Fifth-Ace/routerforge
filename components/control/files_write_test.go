package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ptrInt64(v int64) *int64 { return &v }

func postAdminFileMutation(t *testing.T, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	switch path {
	case "/v1/files/mkdir":
		mutationOnly(handleAdminFileMkdir)(rec, req)
	case "/v1/files/write":
		mutationOnly(handleAdminFileWrite)(rec, req)
	default:
		t.Fatalf("unsupported test path %s", path)
	}
	return rec
}

func TestAdminFileMkdirCreatesSingleDirectory(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "created")

	rec := postAdminFileMutation(t, "/v1/files/mkdir", adminFileMkdirRequest{
		Path:        target,
		ConfirmPath: target,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		t.Fatalf("created directory invalid: info=%v err=%v", info, err)
	}
}

func TestAdminFileMkdirRequiresExactConfirmation(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "created")

	rec := postAdminFileMutation(t, "/v1/files/mkdir", adminFileMkdirRequest{
		Path:        target,
		ConfirmPath: target + "-wrong",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target changed despite confirmation mismatch: %v", err)
	}
}

func TestAdminFileWriteCreateIsAtomicAndMode0600(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config.conf")
	content := "alpha=1\nbeta=2\n"

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:        target,
		ConfirmPath: target,
		Content:     content,
		Create:      true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("content=%q want=%q", string(got), content)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o want=600", info.Mode().Perm())
	}
	leftovers, err := filepath.Glob(filepath.Join(root, ".routerforge-write-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("atomic temp leftovers: %v", leftovers)
	}
}

func TestAdminFileWriteEditRequiresAndChecksPrecondition(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config.conf")
	if err := os.WriteFile(target, []byte("before\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:            target,
		ConfirmPath:     target,
		Content:         "after\n",
		ExpectedSize:    ptrInt64(before.Size()),
		ExpectedMtimeNS: ptrInt64(before.ModTime().UnixNano()),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "after\n" {
		t.Fatalf("content=%q", string(got))
	}
	after, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if after.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%o want=640", after.Mode().Perm())
	}
}

func TestAdminFileWriteRejectsMissingEditPrecondition(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config.conf")
	if err := os.WriteFile(target, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:        target,
		ConfirmPath: target,
		Content:     "after\n",
	})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := os.ReadFile(target)
	if string(got) != "before\n" {
		t.Fatalf("file changed without precondition: %q", string(got))
	}
}

func TestAdminFileWriteRejectsStaleEdit(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config.conf")
	if err := os.WriteFile(target, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}

	staleSize := before.Size()
	staleTime := before.ModTime().UnixNano()

	if err := os.WriteFile(target, []byte("changed elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(target, now, now); err != nil {
		t.Fatal(err)
	}

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:            target,
		ConfirmPath:     target,
		Content:         "ours\n",
		ExpectedSize:    &staleSize,
		ExpectedMtimeNS: &staleTime,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := os.ReadFile(target)
	if string(got) != "changed elsewhere\n" {
		t.Fatalf("stale write modified file: %q", string(got))
	}
}

func TestAdminFileWriteRejectsLeafSymlink(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	realFile := filepath.Join(root, "real.conf")
	link := filepath.Join(root, "link.conf")
	if err := os.WriteFile(realFile, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realFile, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	info, err := os.Stat(link)
	if err != nil {
		t.Fatal(err)
	}

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:            link,
		ConfirmPath:     link,
		Content:         "after\n",
		ExpectedSize:    ptrInt64(info.Size()),
		ExpectedMtimeNS: ptrInt64(info.ModTime().UnixNano()),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := os.ReadFile(realFile)
	if string(got) != "before\n" {
		t.Fatalf("symlink write changed target: %q", string(got))
	}
}

func TestAdminFileWriteRejectsOversizedContentAndUnknownJSON(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config.conf")

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:        target,
		ConfirmPath: target,
		Content:     strings.Repeat("x", int(adminFileWriteContentLimit)+1),
		Create:      true,
	})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status=%d body=%s", rec.Code, rec.Body.String())
	}

	raw := `{"path":"` + target + `","confirm_path":"` + target + `","content":"x","create":true,"unknown":1}`
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/files/write", bytes.NewBufferString(raw))
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	mutationOnly(handleAdminFileWrite)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminFileWriteRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "safe") + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "escape"

	rec := postAdminFileMutation(t, "/v1/files/write", adminFileWriteRequest{
		Path:        target,
		ConfirmPath: target,
		Content:     "x",
		Create:      true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
