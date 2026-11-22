//go:build linux

package lite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

func TestP19LiveSupervisorCrashAndOFFRestart(t *testing.T) {
	binaryPath := os.Getenv("RF_DNS_LITE_TEST_BINARY")
	if binaryPath == "" {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatal("host integration binary missing")
		}
		t.Skip("BLOCKED: live supervisor test requires AF_UNIX and RF_DNS_LITE_TEST_BINARY; mandatory in source CI")
	}
	dir := t.TempDir()
	socket := filepath.Join(dir, "dns.sock")
	probe, e := net.Listen("unix", socket)
	if e != nil {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatal(e)
		}
		t.Skipf("BLOCKED: AF_UNIX unavailable: %v", e)
	}
	probe.Close()
	os.Remove(socket)
	// A local non-loopback fixture satisfies production upstream validation. No
	// public resolver, native ndmc or real netfilter operation is used.
	addresses, e := net.InterfaceAddrs()
	if e != nil {
		t.Fatal(e)
	}
	ip := ""
	for _, a := range addresses {
		p, ok := a.(*net.IPNet)
		if ok && p.IP.To4() != nil && !p.IP.IsLoopback() && p.IP.IsGlobalUnicast() {
			ip = p.IP.String()
			break
		}
	}
	if ip == "" {
		t.Fatal("local IPv4 test interface unavailable")
	}
	tcp, e := net.Listen("tcp", net.JoinHostPort(ip, "0"))
	if e != nil {
		t.Fatal(e)
	}
	udp, e := net.ListenPacket("udp", tcp.Addr().String())
	if e != nil {
		tcp.Close()
		t.Fatal(e)
	}
	handler := dns.HandlerFunc(func(w dns.ResponseWriter, q *dns.Msg) {
		r := new(dns.Msg)
		r.SetReply(q)
		rr, _ := dns.NewRR(q.Question[0].Name + " 0 IN TXT \"SUPERVISED\"")
		r.Answer = []dns.RR{rr}
		w.WriteMsg(r)
	})
	tcpServer := &dns.Server{Listener: tcp, Handler: handler}
	udpServer := &dns.Server{PacketConn: udp, Handler: handler}
	go tcpServer.ActivateAndServe()
	go udpServer.ActivateAndServe()
	defer tcpServer.Shutdown()
	defer udpServer.Shutdown()
	c := fixtureConfig(t)
	c.Upstreams = []dr.Upstream{{ID: "primary", Transport: "udp", Address: tcp.Addr().String()}}
	c.Rules = []dr.Rule{{ID: "r", Domain: "example.org", Match: "suffix", Upstream: "primary", Priority: 100}}
	c.Service.DefaultUpstream = "primary"
	c.Service.CacheBytes = 0
	path := filepath.Join(dir, "config.json")
	store := dr.NewConfigStore(path)
	_, rev, _ := store.Load()
	rev, e = store.Save(c, rev)
	if e != nil {
		t.Fatal(e)
	}
	log, e := os.Create(filepath.Join(dir, "supervisor.log"))
	if e != nil {
		t.Fatal(e)
	}
	defer log.Close()
	cmd := exec.Command(binaryPath, "--config", path, "--socket", socket, "--serve", "--supervise")
	cmd.Stdout = log
	cmd.Stderr = log
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	stopped := false
	defer func() {
		if !stopped {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
		}
	}()
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}}
	readStatus := func() (map[string]any, error) {
		r, e := client.Get("http://local/v1/service/status")
		if e != nil {
			return nil, e
		}
		defer r.Body.Close()
		var body map[string]any
		e = json.NewDecoder(r.Body).Decode(&body)
		return body, e
	}
	waitStatus := func(active bool, oldPID int) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			body, e := readStatus()
			var owner socketOwner
			ownerErr := readState(socket+".owner.json", &owner, 4096)
			if e == nil && ownerErr == nil && body["active"] == active && (oldPID == 0 || owner.PID != oldPID) {
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		log.Seek(0, 0)
		b, _ := io.ReadAll(log)
		t.Fatalf("supervisor state timeout active=%v: %s", active, b)
	}
	post := func(op, revision string) {
		t.Helper()
		b, _ := json.Marshal(map[string]string{"operation": op})
		r, _ := http.NewRequest("POST", "http://local/v1/service/control", bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-RouterForge-Action", "service-control")
		if revision != "" {
			r.Header.Set("If-Match", revision)
		}
		res, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("control %s failed: %s", op, b)
		}
	}
	waitStatus(false, 0)
	post("start", rev)
	waitStatus(true, 0)
	ask(t, c.Service.Listen, "udp", "ordinary.example.org")
	draft := c
	draft.Rules = nil
	draftRev, e := store.Save(draft, rev)
	if e != nil {
		t.Fatal(e)
	}
	var owner socketOwner
	if e = readState(socket+".owner.json", &owner, 4096); e != nil {
		t.Fatal(e)
	}
	if e = syscall.Kill(owner.PID, syscall.SIGKILL); e != nil {
		t.Fatal(e)
	}
	waitStatus(true, owner.PID)
	status, e := readStatus()
	if e != nil || status["applied_revision"] != rev || status["rules"] != float64(1) {
		t.Fatalf("crash activated the draft: %v %v", status, e)
	}
	ask(t, c.Service.Listen, "tcp", "after-crash.example.org")
	post("stop", "")
	waitStatus(false, 0)
	if e = readState(socket+".owner.json", &owner, 4096); e != nil {
		t.Fatal(e)
	}
	syscall.Kill(owner.PID, syscall.SIGKILL)
	waitStatus(false, owner.PID)
	status, _ = readStatus()
	desired := status["desired"].(map[string]any)
	if desired["service_enabled"] != false || desired["interception_enabled"] != false {
		t.Fatal("OFF lost after process crash")
	}
	// Exercise the actual reload hook against a running OFF service.
	hook, e := os.ReadFile("../dns-router-runtime/packaging/40-routerforge-dns-controller.sh")
	if e != nil {
		t.Fatal(e)
	}
	hookText := bytes.ReplaceAll(hook, []byte("/opt/bin/routerforge-dns-router"), []byte(binaryPath))
	hookText = bytes.ReplaceAll(hookText, []byte("/opt/var/run/routerforge-dns-router.sock"), []byte(socket))
	hookCmd := exec.Command("sh")
	hookCmd.Stdin = bytes.NewReader(hookText)
	if out, e := hookCmd.CombinedOutput(); e != nil {
		t.Fatalf("OFF reload hook failed: %v %s", e, out)
	}
	status, _ = readStatus()
	if status["active"] != false || status["interception"] != false {
		t.Fatal("reload hook enabled an OFF service")
	}
	// Maintenance keeps the exact last ON state for the replacement package.
	post("start", draftRev)
	waitStatus(true, 0)
	post("maintenance", "")
	waitStatus(false, 0)
	state, e := (IntentStore{ConfigPath: path}).Load()
	if e != nil || !state.ServiceEnabled || state.AppliedRevision != draftRev {
		t.Fatal("maintenance erased ON", e)
	}
	cmd.Process.Signal(syscall.SIGTERM)
	if e = cmd.Wait(); e != nil {
		t.Fatal(fmt.Errorf("supervisor graceful stop: %w", e))
	}
	stopped = true
}
