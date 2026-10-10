package main

import (
	"strings"
	"testing"
)

// K4S proves the preview stays fail-closed across representative valid input
// ranges. No packet filter commands are executed by these tests.
func TestK4SSafetyMatrix(t *testing.T) {
	cases := []struct {
		name string
		o    KnockOptions
	}{
		{"minimum", KnockOptions{Sequence: [3]int{1, 2, 3}, Target: 4, WindowSeconds: 5, AccessSeconds: 30}},
		{"typical", KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120}},
		{"maximum", KnockOptions{Sequence: [3]int{65532, 65533, 65534}, Target: 65535, WindowSeconds: 300, AccessSeconds: 3600}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := buildKnockRules(tc.o)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Hooked || plan.Applied || plan.StrictSequenceVerified || !plan.DeploymentBlocked || len(plan.DeploymentBlockers) == 0 {
				t.Fatalf("unverified plan was not fail-closed: %+v", plan)
			}
			if len(plan.Commands) == 0 {
				t.Fatal("missing dry-run evidence")
			}
			for _, cmd := range plan.Commands {
				joined := " " + strings.Join(cmd, " ") + " "
				if len(cmd) == 0 || cmd[0] != "iptables" {
					t.Fatalf("unexpected preview command: %v", cmd)
				}
				for _, prohibited := range []string{" -j ACCEPT ", " -I INPUT ", " -A INPUT ", " -I FORWARD ", " -A FORWARD ", " -I PREROUTING ", " -A PREROUTING "} {
					if strings.Contains(joined, prohibited) {
						t.Fatalf("preview contains forbidden operation %q: %v", prohibited, cmd)
					}
				}
			}
		})
	}
}

func TestK4SInvalidOptionsFailWithoutCommands(t *testing.T) {
	base := KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120}
	cases := []struct {
		name string
		edit func(*KnockOptions)
	}{
		{"duplicate-knock", func(o *KnockOptions) { o.Sequence[2] = o.Sequence[0] }},
		{"duplicate-target", func(o *KnockOptions) { o.Target = o.Sequence[1] }},
		{"zero-port", func(o *KnockOptions) { o.Sequence[0] = 0 }},
		{"overflow-port", func(o *KnockOptions) { o.Target = 65536 }},
		{"short-window", func(o *KnockOptions) { o.WindowSeconds = 4 }},
		{"long-window", func(o *KnockOptions) { o.WindowSeconds = 301 }},
		{"short-ttl", func(o *KnockOptions) { o.AccessSeconds = 29 }},
		{"long-ttl", func(o *KnockOptions) { o.AccessSeconds = 3601 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			tc.edit(&o)
			plan, err := buildKnockRules(o)
			if err == nil || len(plan.Commands) != 0 || plan.Hooked || plan.Applied {
				t.Fatalf("invalid input produced executable preview: plan=%+v err=%v", plan, err)
			}
		})
	}
}
