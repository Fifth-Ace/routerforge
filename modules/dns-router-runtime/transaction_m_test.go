package dnsrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeNDMCM struct {
	mu           sync.Mutex
	registered   bool
	addFailed    bool
	removeFailed bool
	commandLog   []string
}

func (f *fakeNDMCM) Command(_ context.Context, c string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commandLog = append(f.commandLog, c)
	add, rm := trialCommandsM()
	switch c {
	case "show ip name-server":
		before := `{"server":[{"address":"192.0.2.1","port":53,"domain":"","global":0}],"prompt":"(config)"}`
		if f.registered {
			return `{"server":[{"address":"192.0.2.1","port":53,"domain":"","global":0},{"address":"192.168.10.1","port":55358,"domain":"dpr2m.test","global":0}],"prompt":"(config)"}`, nil
		}
		return before, nil
	case add:
		f.registered = true
		if f.addFailed {
			return "", errors.New("injected partial add")
		}
		return "OK", nil
	case rm:
		if f.removeFailed {
			return "", errors.New("injected remove failure")
		}
		f.registered = false
		return "OK", nil
	default:
		return "", errors.New("unexpected command")
	}
}

type fakeProbeM struct {
	fail  bool
	calls int
}

func (f *fakeProbeM) Check(context.Context) error {
	f.calls++
	if f.fail {
		return errors.New("injected probe failure")
	}
	return nil
}

type fakeGuardM struct{ finishes int }

func (f *fakeGuardM) Finish() error { f.finishes++; return nil }
func readyConfigM() Config {
	c := lConfig()
	c.Upstreams[0].Address = mUpstream
	c.Rules = []Rule{{ID: "m_probe", Domain: mZone, Match: "suffix", Upstream: c.Upstreams[0].ID, Priority: 1}}
	return c
}
func makeTrialM(t *testing.T, executor *fakeNDMCM, probe *fakeProbeM, guard *fakeGuardM) (*NDMCTrialM, string) {
	t.Helper()
	store := NewConfigStore(filepath.Join(t.TempDir(), "dns-router.json"))
	_, rev, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	rev, e = store.Save(readyConfigM(), rev)
	if e != nil {
		t.Fatal(e)
	}
	runner := &NDMCTrialM{Store: store, Executor: executor, Probe: probe, Ready: func(r string) bool { return r == rev }, Arm: func(context.Context, string) (trialGuardM, error) { return guard, nil }}
	return runner, rev
}
func TestDPR2MFullTrialAndRollback(t *testing.T) {
	driver := &fakeNDMCM{}
	probe := &fakeProbeM{}
	guard := &fakeGuardM{}
	runner, rev := makeTrialM(t, driver, probe, guard)
	result, err := runner.Run(context.Background(), rev)
	if err != nil || result.State != "pass" || !result.AddVerified || !result.UDPAndTCPVerified || !result.BaselineRestored || !result.WatchdogArmed || result.DurableChange || driver.registered || guard.finishes != 1 || probe.calls != 1 {
		t.Fatalf("result=%+v err=%v guard=%+v registered=%t", result, err, guard, driver.registered)
	}
	if len(driver.commandLog) != 5 {
		t.Fatalf("commands %+v", driver.commandLog)
	}
}
func TestDPR2MFailureRollback(t *testing.T) {
	for _, tc := range []struct {
		name      string
		addFail   bool
		probeFail bool
	}{{"partial_add", true, false}, {"probe_failure", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			driver := &fakeNDMCM{addFailed: tc.addFail}
			probe := &fakeProbeM{fail: tc.probeFail}
			guard := &fakeGuardM{}
			runner, rev := makeTrialM(t, driver, probe, guard)
			res, err := runner.Run(context.Background(), rev)
			if err == nil || res.State != "rolled_back_after_failure" || !res.BaselineRestored || driver.registered || guard.finishes != 1 {
				t.Fatalf("res=%+v err=%v", res, err)
			}
		})
	}
}
func TestDPR2MAmbiguousRollbackDoesNotAckGuard(t *testing.T) {
	driver := &fakeNDMCM{removeFailed: true}
	probe := &fakeProbeM{}
	guard := &fakeGuardM{}
	runner, rev := makeTrialM(t, driver, probe, guard)
	res, err := runner.Run(context.Background(), rev)
	if err == nil || res.State != "ambiguous" || res.BaselineRestored || guard.finishes != 0 || !driver.registered {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}
func TestDPR2MSafetyGates(t *testing.T) {
	driver := &fakeNDMCM{}
	probe := &fakeProbeM{}
	guard := &fakeGuardM{}
	runner, rev := makeTrialM(t, driver, probe, guard)
	if _, err := runner.Run(context.Background(), "sha256:stale"); !errors.Is(err, ErrStale) {
		t.Fatalf("stale: %v", err)
	}
	if len(driver.commandLog) != 0 {
		t.Fatal("stale run touched ndmc")
	}
	runner.Ready = func(string) bool { return false }
	if _, err := runner.Run(context.Background(), rev); err == nil {
		t.Fatal("inactive listener accepted")
	}
	if len(driver.commandLog) != 0 {
		t.Fatal("inactive listener touched ndmc")
	}
	runner.Ready = func(string) bool { return true }
	driver.registered = true
	if _, err := runner.Run(context.Background(), rev); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing zone: %v", err)
	}
	if guard.finishes != 0 {
		t.Fatal("guard armed for preexisting zone")
	}
	c := readyConfigM()
	c.Upstreams[0].Address = "1.1.1.1:53"
	if exactTrialPlanM(c) == nil {
		t.Fatal("accepted non-lab upstream")
	}
	c = readyConfigM()
	c.Rules[0].Domain = "example.com"
	if exactTrialPlanM(c) == nil {
		t.Fatal("accepted arbitrary zone")
	}
}
func TestDPR2MAPIRequiresExplicitCapabilityAndCAS(t *testing.T) {
	driver := &fakeNDMCM{}
	probe := &fakeProbeM{}
	guard := &fakeGuardM{}
	runner, rev := makeTrialM(t, driver, probe, guard)
	handler := NewAPI(runner.Store, "")
	runRequest := func(h string, ver string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/v1/keenetic/domain-trial", bytes.NewBufferString(`{"operation":"trial"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-RouterForge-Action", h)
		req.Header.Set("If-Match", ver)
		w := httptest.NewRecorder()
		handler.Handler().ServeHTTP(w, req)
		return w
	}
	if w := runRequest("ndmc-trial", rev); w.Code != 503 {
		t.Fatalf("capability disabled: %d %s", w.Code, w.Body.String())
	}
	handler.TrialM = runner
	if w := runRequest("wrong", rev); w.Code != 403 {
		t.Fatalf("action: %d", w.Code)
	}
	if w := runRequest("ndmc-trial", "sha256:stale"); w.Code != 409 {
		t.Fatalf("stale: %d %s", w.Code, w.Body.String())
	}
	if w := runRequest("ndmc-trial", rev); w.Code != 200 {
		t.Fatalf("trial: %d %s", w.Code, w.Body.String())
	} else {
		var res TrialResultM
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || res.State != "pass" || !res.BaselineRestored {
			t.Fatalf("res=%+v err=%v", res, err)
		}
	}
}
func TestDPR2MInventoryStrictJSON(t *testing.T) {
	for _, raw := range []string{"{}", "not-json", "null"} {
		if _, err := absentFromSnapshotM(raw); err == nil {
			t.Fatalf("accepted malformed inventory %q", raw)
		}
	}
	if yes, err := absentFromSnapshotM(`{"server":[]}`); err != nil || !yes {
		t.Fatalf("empty inventory yes=%t err=%v", yes, err)
	}
}

func TestDPR2MWatchdogWorkerRealSubprocessMockNDMC(t *testing.T) {
	// The worker executes a separate ndmc command, not the in-process fake.
	// A no-longer-existing parent simulates a dead RouterForge daemon.
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "zone-present")
	if err := os.WriteFile(state, []byte("1"), 0600); err != nil {
		t.Fatal(err)
	}
	mock := filepath.Join(binDir, "ndmc")
	script := fmt.Sprintf(`#!/bin/sh
[ "$1" = '-c' ] || exit 2
case "$2" in
  'no ip name-server 192.168.10.1:55358 dpr2m.test') rm -f '%s'; echo OK;;
  'show ip name-server')
     if [ -f '%s' ]; then echo '{"server":[{"address":"192.168.10.1","port":55358,"domain":"dpr2m.test"}]}';
     else echo '{"server":[]}'; fi;;
  *) exit 3;;
esac
`, state, state)
	if err := os.WriteFile(mock, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	journal := trialJournalM{Parent: 9999999, Baseline: "{\"server\":[]}\n", Deadline: time.Now().Add(30 * time.Second).Unix()}
	b, _ := json.Marshal(journal)
	if err := os.WriteFile(filepath.Join(dir, "journal.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := NDMCWatchdogWorkerM(dir); err != nil {
		t.Fatalf("worker: %v", err)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatalf("state after rollback err=%v", err)
	}
	result, err := os.ReadFile(filepath.Join(dir, "result.txt"))
	if err != nil || string(result) != "WATCHDOG_ROLLBACK_PASS\n" {
		t.Fatalf("watchdog result %s err=%v", result, err)
	}
}
