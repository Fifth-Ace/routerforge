package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSmartApplyReferencedNames(t *testing.T) {
	cfg := `NFQWS_BASE_ARGS="--blob=tls:@/opt/etc/nfqws2/blobs/tls.bin --blob=q:@/opt/etc/nfqws2/blobs/q.bin"
NFQWS_ARGS="--hostlist=/opt/etc/nfqws2/lists/user.list --hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list"
NFQWS_ARGS_IPSET="--ipset=/opt/etc/nfqws2/lists/ipset.list"`
	lists, blobs := smartApplyReferencedNames(cfg)
	for _, name := range []string{"user.list", "exclude.list", "ipset.list"} {
		if !lists[name] {
			t.Fatalf("missing list ref %s", name)
		}
	}
	for _, name := range []string{"tls.bin", "q.bin"} {
		if !blobs[name] {
			t.Fatalf("missing blob ref %s", name)
		}
	}
}

func TestSmartApplyDecodeListDependency(t *testing.T) {
	data := []byte("example.com\n")
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	deps, total, err := smartApplyDecodeDependencies("list", []smartApplyDependency{{
		Name: "user.list", ContentBase64: base64.StdEncoding.EncodeToString(data), ExpectedSHA256: hash,
	}}, map[string]bool{"user.list": true})
	if err != nil {
		t.Fatal(err)
	}
	if total != len(data) || len(deps) != 1 || deps[0].hash != hash {
		t.Fatal("decoded dependency mismatch")
	}
}

func TestSmartApplyRejectsUnreferencedDependency(t *testing.T) {
	data := []byte{1, 2, 3}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	_, _, err := smartApplyDecodeDependencies("blob", []smartApplyDependency{{
		Name: "extra.bin", ContentBase64: base64.StdEncoding.EncodeToString(data), ExpectedSHA256: hash,
	}}, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), "unreferenced") {
		t.Fatalf("expected unreferenced rejection: %v", err)
	}
}

func TestSmartApplyRejectsDuplicateDependency(t *testing.T) {
	data := []byte("example.com\n")
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	dep := smartApplyDependency{Name: "user.list", ContentBase64: base64.StdEncoding.EncodeToString(data), ExpectedSHA256: hash}
	_, _, err := smartApplyDecodeDependencies("list", []smartApplyDependency{dep, dep}, map[string]bool{"user.list": true})
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("expected duplicate rejection: %v", err)
	}
}
