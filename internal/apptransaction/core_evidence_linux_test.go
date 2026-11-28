//go:build linux

package apptransaction

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func coreTestSnapshot(on bool) map[string]any {
	return map[string]any{"enabled": on, "running": on, "listen": "127.0.0.1:6061", "slow_ms": 750, "error": "", "mode": "loopback-only"}
}
func TestCoreEvidenceNativePeerAndIntent(t *testing.T) {
	d := t.TempDir()
	dir := filepath.Join(d, "evidence")
	uid := uint32(os.Geteuid())
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	marker, config := filepath.Join(d, "enabled"), filepath.Join(d, "config")
	if e = os.WriteFile(marker, nil, 0600); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(config, []byte("slow_ms=750\n"), 0600); e != nil {
		t.Fatal(e)
	}
	s, e := startCoreEvidence(dir, uid, "old", func() map[string]any { return coreTestSnapshot(true) })
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if x, e := startCoreEvidence(dir, uid, "other", func() map[string]any { return coreTestSnapshot(true) }); e == nil {
		x.Close()
		t.Fatal("duplicate listener accepted")
	}
	socket := filepath.Join(dir, "core.sock")
	probe := func(v string) (CoreProfilingEvidence, error) {
		return probeCoreProfilingEvidence(context.Background(), socket, exe, marker, config, uid, v)
	}
	a, e := probe("old")
	if e != nil || !a.Enabled || a.MarkerSHA256 == "" || a.ConfigSHA256 == "" {
		t.Fatalf("before: %+v %v", a, e)
	}
	if _, e = probe("new"); e == nil {
		t.Fatal("wrong runtime accepted")
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	req, _ := http.NewRequest("POST", "http://local"+coreEvidencePath, nil)
	resp, e := tr.RoundTrip(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("POST accepted: %d", resp.StatusCode)
	}
	if e = s.Close(); e != nil {
		t.Fatal(e)
	}
	s2, e := startCoreEvidence(dir, uid, "new", func() map[string]any { return coreTestSnapshot(true) })
	if e != nil {
		t.Fatal(e)
	}
	defer s2.Close()
	b, e := probe("new")
	if e != nil {
		t.Fatal(e)
	}
	if e = CompareCoreProfilingIntent(a, b, "new"); e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(config, []byte("slow_ms=900\n"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = probe("new"); e == nil {
		t.Fatal("runtime/config mismatch accepted")
	}
	if e = os.WriteFile(config, []byte("slow_ms=750\n# drift\n"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e = probe("new")
	if e != nil {
		t.Fatal(e)
	}
	if CompareCoreProfilingIntent(a, b, "new") == nil {
		t.Fatal("configuration bytes changed")
	}

	if e = os.Remove(marker); e != nil {
		t.Fatal(e)
	}
	if _, e = probe("new"); e == nil {
		t.Fatal("lost marker accepted")
	}
	if e = os.Symlink(config, marker); e != nil {
		t.Fatal(e)
	}
	if _, e = probe("new"); e == nil {
		t.Fatal("symlink marker accepted")
	}
}
func TestCoreEvidencePayloadRejectsUnhealthy(t *testing.T) {
	for _, mutate := range []func(map[string]any){func(p map[string]any) { p["running"] = false }, func(p map[string]any) { p["error"] = "bind failed" }, func(p map[string]any) { p["listen"] = "0.0.0.0:6061" }, func(p map[string]any) { p["listen"] = "127.0.0.1:0" }, func(p map[string]any) { delete(p, "enabled") }, func(p map[string]any) { p["enabled"] = nil }} {
		p := coreTestSnapshot(true)
		mutate(p)
		b, _ := json.Marshal(map[string]any{"ok": true, "version": "v", "module_abi": "v1", "web_bound": true, "profiling": p})
		if _, e := decodeCoreEvidence(b, "v"); e == nil {
			t.Fatalf("accepted %+v", p)
		}
	}
	p := coreTestSnapshot(false)
	b, _ := json.Marshal(map[string]any{"ok": true, "version": "v", "module_abi": "v1", "web_bound": true, "profiling": p})
	if _, e := decodeCoreEvidence(b, "v"); e != nil {
		t.Fatal(e)
	}
}
func TestCoreEvidenceRejectsUnsafeDir(t *testing.T) {
	d := t.TempDir()
	dir := filepath.Join(d, "evidence")
	if e := os.Symlink(d, dir); e != nil {
		t.Fatal(e)
	}
	if s, e := startCoreEvidence(dir, uint32(os.Geteuid()), "v", func() map[string]any { return coreTestSnapshot(false) }); e == nil {
		s.Close()
		t.Fatal("symlink dir accepted")
	}
}

// A real subprocess exercises a stale Unix socket and lock released by the
// kernel, not just a graceful Close in the test's own process.
func TestCoreEvidenceProcessHelper(t *testing.T) {
	if len(os.Args) < 5 || os.Args[len(os.Args)-3] != "RF_CORE_EVIDENCE_HELPER" {
		return
	}
	dir, version := os.Args[len(os.Args)-2], os.Args[len(os.Args)-1]
	s, e := startCoreEvidence(dir, uint32(os.Geteuid()), version, func() map[string]any { return coreTestSnapshot(false) })
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	time.Sleep(40 * time.Second)
}
func TestCoreEvidenceAbruptRestart(t *testing.T) {
	d := t.TempDir()
	dir := filepath.Join(d, "evidence")
	socket := filepath.Join(dir, "core.sock")
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	var child *exec.Cmd
	stop := func() {
		if child != nil {
			_ = child.Process.Kill()
			_ = child.Wait()
			child = nil
		}
	}
	defer stop()
	probe := func(v string) (CoreProfilingEvidence, error) {
		return probeCoreProfilingEvidence(context.Background(), socket, exe, filepath.Join(d, "marker"), filepath.Join(d, "config"), uint32(os.Geteuid()), v)
	}
	start := func(v string) CoreProfilingEvidence {
		child = exec.Command(exe, "-test.run=^TestCoreEvidenceProcessHelper$", "--", "RF_CORE_EVIDENCE_HELPER", dir, v)
		child.Stdout = os.Stderr
		child.Stderr = os.Stderr
		if e := child.Start(); e != nil {
			child = nil
			t.Fatal(e)
		}
		until := time.Now().Add(8 * time.Second)
		for time.Now().Before(until) {
			got, e := probe(v)
			if e == nil {
				return got
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatal("child evidence did not become available")
		return CoreProfilingEvidence{}
	}
	a := start("old")
	if a.PeerPID != child.Process.Pid {
		t.Fatal("wrong child PID")
	}
	stop()
	if _, e := probe("old"); e == nil {
		t.Fatal("dead Core accepted")
	}
	if info, e := os.Lstat(socket); e != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatal("expected stale socket", e)
	}
	b := start("new")
	if b.PeerPID != child.Process.Pid {
		t.Fatal("wrong restarted PID")
	}
	if e := CompareCoreProfilingIntent(a, b, "new"); e != nil {
		t.Fatal(e)
	}
}
