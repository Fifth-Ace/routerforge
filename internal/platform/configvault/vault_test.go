package configvault

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func testStore(t *testing.T, maxSnapshots int) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "managed")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := New(Options{
		Root:             filepath.Join(root, "vault"),
		AllowedRoots:     []string{source},
		MaxArtifactBytes: 1024,
		MaxSnapshotBytes: 4096,
		MaxStoreBytes:    1 << 20,
		MaxSnapshots:     maxSnapshots,
	})
	if err != nil {
		t.Fatal(err)
	}
	return store, source
}

func TestCaptureListAndDeduplicateObjects(t *testing.T) {
	store, source := testStore(t, 8)
	path := filepath.Join(source, "app.conf")
	if err := os.WriteFile(path, []byte("enabled=true\n"), 0600); err != nil {
		t.Fatal(err)
	}

	first, err := store.Capture(CaptureRequest{
		Component: "admin",
		Reason:    "before change",
		Artifacts: []ArtifactSpec{{ID: "app", Path: path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.Capture(CaptureRequest{
		Component: "admin",
		Reason:    "same content",
		Artifacts: []ArtifactSpec{{ID: "app", Path: path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("snapshot ids must be unique")
	}
	if first.Artifacts[0].SHA256 != second.Artifacts[0].SHA256 {
		t.Fatal("same content must use the same object checksum")
	}

	objects, err := os.ReadDir(store.objectsDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 deduplicated object, got %d", len(objects))
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(list))
	}
}

func TestCaptureRejectsOutsideAllowedRoot(t *testing.T) {
	store, _ := testStore(t, 8)
	outside := filepath.Join(t.TempDir(), "outside.conf")
	if err := os.WriteFile(outside, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := store.Capture(CaptureRequest{
		Component: "dns",
		Artifacts: []ArtifactSpec{{ID: "outside", Path: outside}},
	})
	if err == nil {
		t.Fatal("expected outside-root rejection")
	}
}

func TestCaptureRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not reliably available")
	}
	store, source := testStore(t, 8)
	target := filepath.Join(source, "target.conf")
	link := filepath.Join(source, "link.conf")
	if err := os.WriteFile(target, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	_, err := store.Capture(CaptureRequest{
		Component: "dns",
		Artifacts: []ArtifactSpec{{ID: "link", Path: link}},
	})
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestLastWorkingIsPinnedDuringRetention(t *testing.T) {
	store, source := testStore(t, 2)
	path := filepath.Join(source, "config")
	if err := os.WriteFile(path, []byte("one"), 0600); err != nil {
		t.Fatal(err)
	}
	first, err := store.Capture(CaptureRequest{Component: "admin", Artifacts: []ArtifactSpec{{ID: "cfg", Path: path}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkLastWorking(first.ID); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond)
	if err := os.WriteFile(path, []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := store.Capture(CaptureRequest{Component: "admin", Artifacts: []ArtifactSpec{{ID: "cfg", Path: path}}})
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond)
	if err := os.WriteFile(path, []byte("three"), 0600); err != nil {
		t.Fatal(err)
	}
	third, err := store.Capture(CaptureRequest{Component: "admin", Artifacts: []ArtifactSpec{{ID: "cfg", Path: path}}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.Get(first.ID); err != nil {
		t.Fatalf("last working snapshot must survive retention: %v", err)
	}
	if _, err := store.Get(second.ID); !os.IsNotExist(err) {
		t.Fatalf("old unpinned snapshot should be pruned, got %v", err)
	}
	if _, err := store.Get(third.ID); err != nil {
		t.Fatalf("new snapshot must survive retention: %v", err)
	}
	state, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.LastWorking != first.ID {
		t.Fatalf("unexpected last working id %q", state.LastWorking)
	}
}

func TestArtifactAndSnapshotLimits(t *testing.T) {
	store, source := testStore(t, 8)
	path := filepath.Join(source, "large")
	if err := os.WriteFile(path, make([]byte, 1025), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Capture(CaptureRequest{
		Component: "admin",
		Artifacts: []ArtifactSpec{{ID: "large", Path: path}},
	}); err == nil {
		t.Fatal("expected artifact size limit")
	}
}

func TestReadArtifactVerifiesStoredObject(t *testing.T) {
	store, source := testStore(t, 8)
	path := filepath.Join(source, "restore.conf")
	content := []byte("restore=true\n")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.Capture(CaptureRequest{
		Component: "admin",
		Artifacts: []ArtifactSpec{{ID: "restore", Path: path}},
	})
	if err != nil {
		t.Fatal(err)
	}

	record, restored, err := store.ReadArtifact(manifest.ID, "restore")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "restore" || string(restored) != string(content) {
		t.Fatalf("unexpected restored artifact: %+v %q", record, restored)
	}

	object := store.objectPath(record.SHA256)
	if err := os.WriteFile(object, []byte("tampered\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ReadArtifact(manifest.ID, "restore"); err == nil {
		t.Fatal("tampered content object must be rejected")
	}
}

func TestReadArtifactRejectsUnknownArtifact(t *testing.T) {
	store, source := testStore(t, 8)
	path := filepath.Join(source, "known.conf")
	if err := os.WriteFile(path, []byte("known\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest, err := store.Capture(CaptureRequest{
		Component: "admin",
		Artifacts: []ArtifactSpec{{ID: "known", Path: path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.ReadArtifact(manifest.ID, "missing"); !os.IsNotExist(err) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestCaptureEmptyBaseline(t *testing.T) {
	store, _ := testStore(t, 8)
	manifest, err := store.Capture(CaptureRequest{
		Component: "admin",
		Reason:    "empty baseline",
		Artifacts: []ArtifactSpec{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Artifacts) != 0 || manifest.TotalBytes != 0 {
		t.Fatalf("unexpected empty snapshot: %+v", manifest)
	}
	loaded, err := store.Get(manifest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Artifacts) != 0 {
		t.Fatalf("stored empty baseline gained artifacts: %+v", loaded.Artifacts)
	}
}
