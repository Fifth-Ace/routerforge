package configvault

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestE062EncryptedTransport(t *testing.T) {
	root := t.TempDir()
	managed := filepath.Join(root, "managed")
	if e := os.Mkdir(managed, 0700); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(managed, "logging.json")
	secret := []byte("private fixture secret\n")
	if e := os.WriteFile(file, secret, 0600); e != nil {
		t.Fatal(e)
	}
	v, e := New(Options{Root: filepath.Join(root, "vault"), AllowedRoots: []string{managed}})
	if e != nil {
		t.Fatal(e)
	}
	m, e := v.Capture(CaptureRequest{Component: "admin", Version: "test", Artifacts: []ArtifactSpec{{ID: "x", Path: file}}})
	if e != nil {
		t.Fatal(e)
	}
	key := strings.Repeat("a1", 32)
	blob, e := v.ExportEncrypted(m.ID, key)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(blob, secret) {
		t.Fatal("plaintext")
	}
	blob2, e := v.ExportEncrypted(m.ID, key)
	if e != nil || bytes.Equal(blob, blob2) {
		t.Fatal("nonce reuse")
	}
	if e = os.WriteFile(file, []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = v.ImportEncrypted(blob, strings.Repeat("b2", 32)); e == nil {
		t.Fatal("wrong key accepted")
	}
	bad := append([]byte(nil), blob...)
	bad[len(bad)-1] ^= 1
	if _, e = v.ImportEncrypted(bad, key); e == nil {
		t.Fatal("tamper accepted")
	}
	list, e := v.List()
	if e != nil || len(list) != 1 {
		t.Fatal("invalid import wrote state")
	}
	n, e := v.ImportEncrypted(blob, key)
	if e != nil || n.ID == m.ID || n.Version != "test" {
		t.Fatalf("import: %v", e)
	}
	_, data, e := v.ReadArtifact(n.ID, "x")
	if e != nil || !bytes.Equal(data, secret) {
		t.Fatal("readback")
	}
	current, _ := os.ReadFile(file)
	if string(current) != "new" {
		t.Fatal("managed file changed")
	}
}
func TestE062ImportNoPrune(t *testing.T) {
	root := t.TempDir()
	v, e := New(Options{Root: filepath.Join(root, "vault"), AllowedRoots: []string{root}, MaxSnapshots: 1})
	if e != nil {
		t.Fatal(e)
	}
	m, e := v.Capture(CaptureRequest{Component: "admin", Artifacts: []ArtifactSpec{}})
	if e != nil {
		t.Fatal(e)
	}
	key := strings.Repeat("c3", 32)
	blob, e := v.ExportEncrypted(m.ID, key)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = v.ImportEncrypted(blob, key); e == nil {
		t.Fatal("limit not enforced")
	}
	if _, e = v.Get(m.ID); e != nil {
		t.Fatal("snapshot evicted")
	}
}
