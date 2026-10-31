package lite

import (
	"context"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type fakeNDMC struct {
	lines     map[string]bool
	commands  []string
	failAt    int
	mutations int
}

func (f *fakeNDMC) Command(_ context.Context, c string) (string, error) {
	f.commands = append(f.commands, c)
	if c == "show running-config" {
		lines := []string{}
		for l := range f.lines {
			lines = append(lines, l)
		}
		sort.Strings(lines)
		return "hostname DSTest\n!\ndns-proxy\n    " + strings.Join(lines, "\n    ") + "\n!\nip dhcp pool _TEST\n", nil
	}
	f.mutations++
	if f.failAt > 0 && f.mutations == f.failAt {
		return "", errors.New("injected failure")
	}
	if c == "system configuration save" {
		return "Saved", nil
	}
	if strings.HasPrefix(c, "no dns-proxy filter assign host ") {
		fields := strings.Fields(c)
		kind, mac := fields[5], fields[6]
		for l := range f.lines {
			if assignment(l, mac) && strings.Fields(l)[3] == kind {
				delete(f.lines, l)
			}
		}
		return "ok", nil
	}
	if c == "no dns-proxy filter profile "+profileName {
		for l := range f.lines {
			if ownedLine(l) {
				delete(f.lines, l)
			}
		}
		return "ok", nil
	}
	if c == "no dns-proxy filter engine" {
		for l := range f.lines {
			if strings.HasPrefix(l, "filter engine ") {
				delete(f.lines, l)
			}
		}
		return "ok", nil
	}
	if strings.HasPrefix(c, "dns-proxy ") {
		l := strings.TrimPrefix(c, "dns-proxy ")
		fields := strings.Fields(l)
		if strings.HasPrefix(l, "filter assign host ") {
			for old := range f.lines {
				if assignment(old, strings.ToLower(fields[4])) {
					delete(f.lines, old)
				}
			}
		}
		if strings.HasPrefix(l, "filter engine ") {
			for old := range f.lines {
				if strings.HasPrefix(old, "filter engine ") {
					delete(f.lines, old)
				}
			}
		}
		f.lines[l] = true
		return "ok", nil
	}
	return "", errors.New("unexpected command")
}
func newProfileFixture(t *testing.T) (*ProfileManager, *fakeNDMC) {
	t.Helper()
	f := &fakeNDMC{lines: map[string]bool{"filter engine public": true, "filter profile existing": true, "filter profile existing dns53 upstream 9.9.9.9": true, "filter assign host profile 02:11:22:33:44:55 existing": true}}
	return &ProfileManager{Path: filepath.Join(t.TempDir(), "profile.json"), Executor: f}, f
}
func TestProfileConnectDisconnectRestoresExistingAssignment(t *testing.T) {
	p, f := newProfileFixture(t)
	baseline, _ := f.Command(context.Background(), "show running-config")
	if e := p.Connect(context.Background(), "02:11:22:33:44:55", "192.168.10.1:55358", true); e != nil {
		t.Fatal(e)
	}
	if !p.Connected() {
		t.Fatal("ownership missing")
	}
	if e := p.Disconnect(context.Background()); e != nil {
		t.Fatal(e)
	}
	after, _ := f.Command(context.Background(), "show running-config")
	if after != baseline || p.Connected() {
		t.Fatal("baseline was not restored")
	}
}
func TestProfileEachMutationFailureRestoresBaseline(t *testing.T) {
	for failAt := 1; failAt <= 5; failAt++ {
		t.Run(string(rune('0'+failAt)), func(t *testing.T) {
			p, f := newProfileFixture(t)
			baseline, _ := f.Command(context.Background(), "show running-config")
			f.failAt = failAt
			if e := p.Connect(context.Background(), "02:11:22:33:44:55", "192.168.10.1:55358", true); e == nil {
				t.Fatal("injected failure accepted")
			}
			after, _ := f.Command(context.Background(), "show running-config")
			if after != baseline || p.Connected() {
				t.Fatalf("mutation %d left partial DNS state", failAt)
			}
		})
	}
}
func TestProfileReservedNameForeignDriftAndOtherEngine(t *testing.T) {
	p, f := newProfileFixture(t)
	f.lines["filter profile "+profileName] = true
	if e := p.Connect(context.Background(), "02:11:22:33:44:55", "192.168.10.1:55358", false); e == nil || f.mutations != 0 {
		t.Fatal("foreign profile overwritten")
	}
	delete(f.lines, "filter profile "+profileName)
	if e := p.Connect(context.Background(), "02:11:22:33:44:55", "192.168.10.1:55358", false); e != nil {
		t.Fatal(e)
	}
	f.lines["filter profile unrelated"] = true
	n := f.mutations
	if e := p.Disconnect(context.Background()); e == nil || f.mutations != n {
		t.Fatal("foreign DNS changes ignored")
	}
	p, f = newProfileFixture(t)
	delete(f.lines, "filter engine public")
	f.lines["filter engine nextdns"] = true
	if e := p.Connect(context.Background(), "02:11:22:33:44:55", "192.168.10.1:55358", false); e == nil || f.mutations != 0 {
		t.Fatal("other filtering engine overwritten")
	}
}
func TestProfileRefusesUnrecognizedOutputAndInvalidMAC(t *testing.T) {
	if _, e := dnsLines("{\"error\":\"not authorized\"}"); e == nil {
		t.Fatal("invalid config accepted")
	}
	p, f := newProfileFixture(t)
	if e := p.Connect(context.Background(), "aa; reboot", "192.168.10.1:55358", false); e == nil || len(f.commands) != 0 {
		t.Fatal("invalid MAC accepted")
	}
}

func TestProfilePresetAndNonPersistentReboot(t *testing.T) {
	p, f := newProfileFixture(t)
	delete(f.lines, "filter assign host profile 02:11:22:33:44:55 existing")
	f.lines["filter assign host preset 02:11:22:33:44:55 quad9-security"] = true
	delete(f.lines, "filter engine public")
	baseline := map[string]bool{}
	for k, v := range f.lines {
		baseline[k] = v
	}
	if e := p.Connect(context.Background(), "02:11:22:33:44:55", "192.168.10.1:55358", false); e != nil {
		t.Fatal(e)
	}
	for _, c := range f.commands {
		if strings.Contains(c, "filter engine ") {
			t.Fatal("global DNS engine was changed")
		}
	}
	f.lines = baseline // A router reboot restored non-persistent native settings.
	mutations := f.mutations
	p2 := &ProfileManager{Path: p.Path, Executor: f}
	if e := p2.Reconcile(context.Background()); e != nil {
		t.Fatal(e)
	}
	if p2.Connected() || f.mutations != mutations {
		t.Fatal("reboot reconciliation changed native configuration or retained stale ownership")
	}
}
