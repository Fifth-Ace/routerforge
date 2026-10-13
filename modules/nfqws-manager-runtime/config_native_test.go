package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const nf1bNativeSample = `# nfqws2-keenetic style
ISP_INTERFACE="eth3"
NFQWS_BASE_ARGS="--fastpath-workaround=auto
--lua-init=@/opt/etc/nfqws2/lua/zapret-lib.lua"
NFQWS_ARGS="--filter-tcp=443 --payload=tls_client_hello
# HTTPS strategy
--lua-desync=multisplit:pos=1"
NFQWS_ARGS_QUIC="--filter-udp=443 --payload=quic_initial --lua-desync=fake:repeats=5"
NFQWS_ARGS_IPSET="--ipset=/opt/etc/nfqws2/lists/ipset.list"
MODE_LIST="--hostlist=/opt/etc/nfqws2/lists/user.list"
MODE_ALL="--hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list"
MODE_AUTO="$MODE_LIST $MODE_ALL"
NFQWS_EXTRA_ARGS="$MODE_AUTO"
NFQWS_ARGS_CUSTOM="--filter-tcp=80 --lua-desync=multisplit:pos=1 --new --filter-tcp=443 --lua-desync=multisplit:pos=2"
NFQUEUE_NUM=300
USER=nobody
LOG_LEVEL=1
LOG_DEBUG_PATH="@/opt/var/log/nfqws2-debug.log"
`

func TestNF1B2NativeArgvReconstructsModeAndProfiles(t *testing.T) {
	args, profiles, err := nf1bNativeArgv(nf1bNativeSample)
	if err != nil {
		t.Fatal(err)
	}
	joined := " " + strings.Join(args, " ") + " "
	for _, expected := range []string{" --dry-run ", " --qnum=300 ", " --debug=@/opt/var/log/nfqws2-debug.log ", " --lua-init=@/opt/etc/nfqws2/lua/zapret-lib.lua ", " --hostlist=/opt/etc/nfqws2/lists/user.list ", " --hostlist-exclude=/opt/etc/nfqws2/lists/exclude.list ", " --ipset-ip=0.0.0.0 ", " --filter-udp=443 "} {
		if !strings.Contains(joined, expected) {
			t.Errorf("missing %q in %s", expected, joined)
		}
	}
	if !strings.Contains(joined, " # ") || !strings.Contains(joined, " HTTPS ") {
		t.Fatalf("quoted comment is not preserved as actual CLI words: %s", joined)
	}
	if profiles != 6 {
		t.Fatalf("profiles=%d, want 6", profiles)
	}
}

func TestNF1B2NativeDeclinesDynamicShell(t *testing.T) {
	for _, candidate := range []string{
		`NFQWS_ARGS="$(touch /tmp/should-not-exist)"`,
		"NFQWS_ARGS='--filter-tcp=443'\ntouch /tmp/should-not-exist",
		"NFQWS_ARGS='--filter-tcp=443'\nMODE_AUTO='$UNKNOWN'\nNFQWS_EXTRA_ARGS='$MODE_AUTO'",
		"NFQWS_ARGS='--filter-tcp=443'\nNFQWS_ARGS='--filter-tcp=80'",
	} {
		if _, _, err := nf1bNativeArgv(candidate); err == nil {
			t.Errorf("unsafe config accepted: %s", candidate)
		}
	}
}

func TestNF1B2NativeDryRunUsesArgumentVector(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-nfqws2")
	marker := filepath.Join(dir, "marker")
	script := "#!/bin/sh\ncase \"$1\" in --dry-run) printf 'valid args\\n'; exit 0;; *) exit 11;; esac\n"
	if err := os.WriteFile(binary, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	prior := nf1bNativeLocateBinary
	nf1bNativeLocateBinary = func() string { return binary }
	defer func() { nf1bNativeLocateBinary = prior }()
	data := "NFQWS_ARGS='--filter-tcp=443'\nDANGEROUS=$(touch " + marker + ")\n"
	result := nf1bNativeDryRun(data)
	if result.State != "not_performed" {
		t.Fatalf("dynamic code must not run: %+v", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("command substitution executed: %v", err)
	}
	good := nf1bNativeDryRun("NFQWS_ARGS='--filter-tcp=443'\n")
	if good.State != "passed" || good.CheckedProfiles != 1 {
		t.Fatalf("unexpected native result: %+v", good)
	}
}

func TestNF1B2NativeDryRunReturnsBinaryFailure(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "fake-nfqws2")
	if err := os.WriteFile(binary, []byte("#!/bin/sh\necho bad-option >&2\nexit 2\n"), 0755); err != nil {
		t.Fatal(err)
	}
	prior := nf1bNativeLocateBinary
	nf1bNativeLocateBinary = func() string { return binary }
	defer func() { nf1bNativeLocateBinary = prior }()
	result := nf1bNativeDryRun("NFQWS_ARGS='--filter-tcp=443'\n")
	if result.State != "failed" || !strings.Contains(result.Detail, "bad-option") {
		t.Fatalf("unexpected binary failure: %+v", result)
	}
}
