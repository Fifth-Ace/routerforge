package dnsrouter

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const ndmcTextFixtureM = "\x1b[0mserver:\n  address: 1.1.1.1\n  port: 0\n  domain:\n  global: 0\n  service: Dns::Manager\n  interface:\nserver:\n  address: 192.0.2.17\n  port: 853\n  domain: test.example\n  global: 0\n  service: manual\n  interface: ISP\n\x1b[0m"

func TestDPR2MNativeKeeneticCLI(t *testing.T) {
	got, err := canonicalNDMCSnapshotM(ndmcTextFixtureM)
	if err != nil {
		t.Fatal(err)
	}
	if b, e := absentFromSnapshotM(got); e != nil || !b {
		t.Fatalf("absent=%t err=%v", b, e)
	}
	if _, err = canonicalNDMCSnapshotM(got); err != nil {
		t.Fatalf("idempotent JSON baseline: %v", err)
	}
	// Reordering field lines must not cause a false drift.
	swapped := strings.Replace(ndmcTextFixtureM, "  address: 1.1.1.1\n  port: 0\n  domain:", "  domain:\n  port: 0\n  address: 1.1.1.1", 1)
	same, e := canonicalNDMCSnapshotM(swapped)
	if e != nil || same != got {
		t.Fatalf("field order drift: %s / %s (%v)", got, same, e)
	}
	inserted := ndmcTextFixtureM + "\nserver:\n address: 192.168.10.1\n port: 55358\n domain: dpr2m.test\n"
	changed, e := canonicalNDMCSnapshotM(inserted)
	if e != nil || changed == got {
		t.Fatalf("real zone drift: %v", e)
	}
	yes, e := presentInSnapshotM(changed)
	if e != nil || !yes {
		t.Fatalf("present=%t err=%v", yes, e)
	}
}
func TestDPR2MRejectsUntrustedNativeCLI(t *testing.T) {
	cases := []string{
		"server:\n address: 1.1.1.1\n domain: \n evil no ip name-server",
		"server:\n address: 1.1.1.1\n address: 2.2.2.2\n domain:",
		"server:\n domain:",
		"server:\n address: 1.1.1.1\n domain:\n port: 55358Z",
		"error: unknown command",
		"status: failed\nserver:\n address: 1.1.1.1\n domain:",
		"server:\n address: 1.1.1.1\n domain:\nnext-section:",
	}
	for _, raw := range cases {
		if _, err := canonicalNDMCSnapshotM(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
func TestDPR2MNativeCLIInTransactionMock(t *testing.T) {
	drv := &fakeNDMCM{}
	probe := &fakeProbeM{}
	guard := &fakeGuardM{}
	trial, rev := makeTrialM(t, drv, probe, guard)
	trial.Executor = ndmcTextFakeM{drv}
	result, err := trial.Run(context.Background(), rev)
	if err != nil || result.State != "pass" || !result.BaselineRestored {
		t.Fatalf("%+v %v", result, err)
	}
}

type ndmcTextFakeM struct{ inner *fakeNDMCM }

func (f ndmcTextFakeM) Command(ctx context.Context, command string) (string, error) {
	out, e := f.inner.Command(ctx, command)
	if e != nil {
		return out, e
	}
	if command != "show ip name-server" {
		return out, nil
	}
	raw := "server:\n address: 192.0.2.1\n port: 53\n domain:\n global: 0\n"
	if f.inner.registered {
		raw += "server:\n address: 192.168.10.1\n port: 55358\n domain: dpr2m.test\n global: 0\n"
	}
	return raw, nil
}

var _ = errors.New
