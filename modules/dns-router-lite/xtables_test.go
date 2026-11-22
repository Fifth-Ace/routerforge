package lite

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestXTablesRetriesOnlyLockFailures(t *testing.T) {
	for _, tc := range []struct {
		name, msg string
		want      int
		success   bool
	}{{"released", "Another app is currently holding the xtables lock", 3, true}, {"held", "Resource temporarily unavailable", 3, false}, {"permission", "Permission denied", 1, false}, {"syntax", "Bad rule (does a matching rule exist in that chain?)", 1, false}, {"module", "No chain/target/match by that name", 1, false}, {"exit4_other", "exit status 4: unsupported resource", 1, false}} {
		t.Run(tc.name, func(t *testing.T) {
			n := 0
			x := &XTables{Delay: func(context.Context, time.Duration) error { return nil }, Exec: func(_ context.Context, _ string, a ...string) (string, error) {
				n++
				if len(a) < 2 || a[0] != "-w" || a[1] != "1" {
					t.Fatalf("missing bounded wait: %v", a)
				}
				if tc.success && n == 3 {
					return "ok", nil
				}
				return tc.msg, errors.New("exit status 4")
			}}
			_, err := x.Run(context.Background(), "iptables", "-t", "nat", "-S")
			if n != tc.want || (err == nil) != tc.success {
				t.Fatalf("attempts=%d err=%v", n, err)
			}
		})
	}
}
func TestXTablesLegacyWaitAndCancellation(t *testing.T) {
	n := 0
	x := &XTables{Exec: func(_ context.Context, _ string, a ...string) (string, error) {
		n++
		if len(a) > 1 && a[0] == "-w" && a[1] == "1" {
			return "Bad argument `1'", errors.New("exit status 2")
		}
		if a[0] != "-w" {
			t.Fatalf("legacy wait missing: %v", a)
		}
		return "ok", nil
	}}
	if _, e := x.Run(context.Background(), "ip6tables", "-t", "nat", "-S"); e != nil || n != 2 {
		t.Fatalf("compatibility: %d %v", n, e)
	}
	if _, e := x.Run(context.Background(), "ip6tables", "-t", "nat", "-S"); e != nil || n != 3 {
		t.Fatalf("cached capability: %d %v", n, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := x.Run(ctx, "ip6tables", "-S"); !errors.Is(e, context.Canceled) || n != 3 {
		t.Fatal("cancelled request executed")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	x = &XTables{Exec: func(context.Context, string, ...string) (string, error) {
		cancel()
		return "xtables lock", errors.New("busy")
	}}
	if _, e := x.Run(ctx, "iptables", "-S"); !errors.Is(e, context.Canceled) {
		t.Fatalf("retry ignored cancellation: %v", e)
	}
}
func TestXTablesDoesNotRetryIPSetOrRewriteArgs(t *testing.T) {
	x := &XTables{Exec: func(_ context.Context, name string, a ...string) (string, error) {
		if name != "ipset" || strings.Join(a, " ") != "save RF_DNS_LITE4" {
			t.Fatalf("unexpected command %s %v", name, a)
		}
		return "permission denied", errors.New("exit 1")
	}}
	if _, err := x.Run(context.Background(), "ipset", "save", "RF_DNS_LITE4"); err == nil {
		t.Fatal("ignored ipset error")
	}
}
