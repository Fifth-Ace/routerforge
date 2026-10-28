package dnsrouter

import (
	"context"
	"strings"
	"testing"
)

func TestDPR2MANSIDecoratedNDMCInventory(t *testing.T) {
	raw := "\x1b[0m\x1b[?25l\r\n{\n\t\"server\": [{\"address\":\"192.0.2.1\",\"port\":53,\"domain\":\"\"}], \"prompt\":\"(config)\"}\x1b[0m\x1b[?25h\n"
	normalized, err := canonicalNDMCSnapshotM(raw)
	if err != nil {
		t.Fatalf("ANSI decorated inventory: %v", err)
	}
	plain, err := canonicalNDMCSnapshotM(`{"prompt":"(config)","server":[{"domain":"","port":53,"address":"192.0.2.1"}]}`)
	if err != nil || normalized != plain {
		t.Fatalf("canonical mismatch: %q / %q (%v)", normalized, plain, err)
	}
	if absent, err := absentFromSnapshotM(normalized); err != nil || !absent {
		t.Fatalf("absent=%t err=%v", absent, err)
	}
}

func TestDPR2MRejectsBadNDMCInventory(t *testing.T) {
	cases := []string{
		"\x1b]malformed-prefix\x07{\"server\":[]}",
		"untrusted output {\"server\":[]}",
		"{\"server\":null}",
		"{\"server\":{}}",
		"{\"server\":[]}{\"server\":[]}",
		"{\"other\":[]}",
		"{\"server\":[]}-not-csi",
	}
	for _, raw := range cases {
		if _, err := canonicalNDMCSnapshotM(raw); err == nil {
			t.Fatalf("accepted invalid ndmc inventory: %q", raw)
		}
	}
}

func TestDPR2MCanonicalizationDoesNotHideDNSDrift(t *testing.T) {
	before, err := canonicalNDMCSnapshotM(`{"server":[{"address":"192.0.2.1","port":53,"domain":""}]}`)
	if err != nil {
		t.Fatal(err)
	}
	after, err := canonicalNDMCSnapshotM(`{"server":[{"address":"192.0.2.2","port":53,"domain":""}]}`)
	if err != nil || strings.EqualFold(before, after) {
		t.Fatalf("drift undetected: %v", err)
	}
}

// Exercise the complete DSTest-only transaction against an ANSI-decorated ndmc mock.
type decoratedNDMCM struct{ inner *fakeNDMCM }

func (d decoratedNDMCM) Command(ctx context.Context, command string) (string, error) {
	v, e := d.inner.Command(ctx, command)
	if command == "show ip name-server" && e == nil {
		return "\x1b[0m\x1b[?25l" + v + "\x1b[0m\n", nil
	}
	return v, e
}
func TestDPR2MFullTransactionWithANSIInventory(t *testing.T) {
	driver := &fakeNDMCM{}
	probe := &fakeProbeM{}
	guard := &fakeGuardM{}
	runner, rev := makeTrialM(t, driver, probe, guard)
	runner.Executor = decoratedNDMCM{driver}
	result, err := runner.Run(context.Background(), rev)
	if err != nil || result.State != "pass" || !result.BaselineRestored || !result.UDPAndTCPVerified || guard.finishes != 1 || driver.registered {
		t.Fatalf("result=%+v err=%v guard=%d registered=%v", result, err, guard.finishes, driver.registered)
	}
}
