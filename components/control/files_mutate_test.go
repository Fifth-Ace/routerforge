package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func postAdminFileGuardedMutation(t *testing.T, route string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, route, bytes.NewReader(payload))
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	switch route {
	case "/v1/files/move":
		mutationOnly(handleAdminFileMove)(rec, req)
	case "/v1/files/delete":
		mutationOnly(handleAdminFileDelete)(rec, req)
	case "/v1/files/chmod":
		mutationOnly(handleAdminFileChmod)(rec, req)
	default:
		t.Fatalf("unsupported mutation route %s", route)
	}
	return rec
}

func observedFileState(t *testing.T, path string) (int64, int64) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size(), info.ModTime().UnixNano()
}

func TestAdminFileMoveRenamesWithinAllowedRoot(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	source := filepath.Join(root, "before.conf")
	destination := filepath.Join(root, "after.conf")
	if err := os.WriteFile(source, []byte("alpha\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, source)

	rec := postAdminFileGuardedMutation(t, "/v1/files/move", adminFileMoveRequest{
		Source:          source,
		Destination:     destination,
		ConfirmSource:   source,
		ConfirmDest:     destination,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "alpha\n" {
		t.Fatalf("destination content=%q", string(got))
	}
}

func TestAdminFileMoveRejectsWrongConfirmationAndExistingDestination(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.WriteFile(source, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, source)

	rec := postAdminFileGuardedMutation(t, "/v1/files/move", adminFileMoveRequest{
		Source:          source,
		Destination:     destination,
		ConfirmSource:   source + "-wrong",
		ConfirmDest:     destination,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("confirmation status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = postAdminFileGuardedMutation(t, "/v1/files/move", adminFileMoveRequest{
		Source:          source,
		Destination:     destination,
		ConfirmSource:   source,
		ConfirmDest:     destination,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("destination status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminFileMoveRejectsStaleSource(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	source := filepath.Join(root, "source")
	destination := filepath.Join(root, "destination")
	if err := os.WriteFile(source, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, source)
	if err := os.WriteFile(source, []byte("changed elsewhere"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(source, now, now); err != nil {
		t.Fatal(err)
	}

	rec := postAdminFileGuardedMutation(t, "/v1/files/move", adminFileMoveRequest{
		Source:          source,
		Destination:     destination,
		ConfirmSource:   source,
		ConfirmDest:     destination,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("stale move created destination: %v", err)
	}
}

func TestAdminFileMoveRejectsCrossRootAndLeafSymlink(t *testing.T) {
	rootA := t.TempDir()
	rootB := t.TempDir()
	withReadAdminFileRoot(t, rootA)
	adminFileAllowedRoots = []string{rootA, rootB}

	source := filepath.Join(rootA, "source")
	if err := os.WriteFile(source, []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, source)
	destination := filepath.Join(rootB, "destination")

	rec := postAdminFileGuardedMutation(t, "/v1/files/move", adminFileMoveRequest{
		Source:          source,
		Destination:     destination,
		ConfirmSource:   source,
		ConfirmDest:     destination,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-root status=%d body=%s", rec.Code, rec.Body.String())
	}

	realFile := filepath.Join(rootA, "real")
	link := filepath.Join(rootA, "link")
	if err := os.WriteFile(realFile, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realFile, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	linkSize := linkInfo.Size()
	linkMtime := linkInfo.ModTime().UnixNano()
	linkDestination := filepath.Join(rootA, "moved-link")
	rec = postAdminFileGuardedMutation(t, "/v1/files/move", adminFileMoveRequest{
		Source:          link,
		Destination:     linkDestination,
		ConfirmSource:   link,
		ConfirmDest:     linkDestination,
		ExpectedSize:    &linkSize,
		ExpectedMtimeNS: &linkMtime,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("symlink status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminFileDeleteRemovesFileAndEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)

	for _, target := range []string{
		filepath.Join(root, "file"),
		filepath.Join(root, "empty"),
	} {
		if filepath.Base(target) == "file" {
			if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		size, mtime := observedFileState(t, target)
		rec := postAdminFileGuardedMutation(t, "/v1/files/delete", adminFileDeleteRequest{
			Path:            target,
			ConfirmPath:     target,
			ExpectedSize:    &size,
			ExpectedMtimeNS: &mtime,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("target=%s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
		if _, err := os.Lstat(target); !os.IsNotExist(err) {
			t.Fatalf("target still exists: %s err=%v", target, err)
		}
	}
}

func TestAdminFileDeleteRejectsNonEmptyDirectoryAndAllowedRoot(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	directory := filepath.Join(root, "nonempty")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "child"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, directory)

	rec := postAdminFileGuardedMutation(t, "/v1/files/delete", adminFileDeleteRequest{
		Path:            directory,
		ConfirmPath:     directory,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("nonempty status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatalf("nonempty directory changed: %v", err)
	}

	rootSize, rootMtime := observedFileState(t, root)
	rec = postAdminFileGuardedMutation(t, "/v1/files/delete", adminFileDeleteRequest{
		Path:            root,
		ConfirmPath:     root,
		ExpectedSize:    &rootSize,
		ExpectedMtimeNS: &rootMtime,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("root status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminFileDeleteRequiresPreconditionAndRejectsStaleObject(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "file")
	if err := os.WriteFile(target, []byte("before"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := postAdminFileGuardedMutation(t, "/v1/files/delete", adminFileDeleteRequest{
		Path:        target,
		ConfirmPath: target,
	})
	if rec.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing precondition status=%d body=%s", rec.Code, rec.Body.String())
	}

	size, mtime := observedFileState(t, target)
	if err := os.WriteFile(target, []byte("changed"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(target, now, now); err != nil {
		t.Fatal(err)
	}
	rec = postAdminFileGuardedMutation(t, "/v1/files/delete", adminFileDeleteRequest{
		Path:            target,
		ConfirmPath:     target,
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale status=%d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("stale delete removed target: %v", err)
	}
}

func TestAdminFileChmodChangesOnlyPermissionBits(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, target)

	rec := postAdminFileGuardedMutation(t, "/v1/files/chmod", adminFileChmodRequest{
		Path:            target,
		ConfirmPath:     target,
		Mode:            "0640",
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode=%o want=640", info.Mode().Perm())
	}
}

func TestAdminFileChmodRejectsSpecialBitsAndLeafSymlink(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "config")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	size, mtime := observedFileState(t, target)

	rec := postAdminFileGuardedMutation(t, "/v1/files/chmod", adminFileChmodRequest{
		Path:            target,
		ConfirmPath:     target,
		Mode:            "4755",
		ExpectedSize:    &size,
		ExpectedMtimeNS: &mtime,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("special mode status=%d body=%s", rec.Code, rec.Body.String())
	}

	link := filepath.Join(root, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	linkSize := linkInfo.Size()
	linkMtime := linkInfo.ModTime().UnixNano()
	rec = postAdminFileGuardedMutation(t, "/v1/files/chmod", adminFileChmodRequest{
		Path:            link,
		ConfirmPath:     link,
		Mode:            "0644",
		ExpectedSize:    &linkSize,
		ExpectedMtimeNS: &linkMtime,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("symlink status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminFileGuardedMutationsRejectUnknownJSON(t *testing.T) {
	root := t.TempDir()
	withReadAdminFileRoot(t, root)
	target := filepath.Join(root, "file")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/v1/files/delete",
		bytes.NewBufferString(`{"path":"`+target+`","confirm_path":"`+target+`","unknown":true}`),
	)
	req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
	mutationOnly(handleAdminFileDelete)(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
