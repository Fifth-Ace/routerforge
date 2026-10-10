package main

import (
	"strings"
	"testing"
)

// K4R is a negative parity gate: the current generated iptables plan must NOT
// be marked strict, since stale recent-list state can survive interleaving.
// This test does not execute a firewall command or certify kernel behavior.
func TestK4RNegativeParityGate(t *testing.T) {
	plan, err := buildKnockRules(KnockOptions{
		Sequence:      [3]int{41001, 41002, 41003},
		Target:        2222,
		WindowSeconds: 30,
		AccessSeconds: 120,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hooked || plan.Applied || plan.StrictSequenceVerified || !plan.DeploymentBlocked {
		t.Fatalf("unproven packet semantics must block deployment: %+v", plan)
	}
	if len(plan.DeploymentBlockers) == 0 {
		t.Fatal("missing deployment blockers")
	}

	var sets, removes, hooks, accepts int
	for _, args := range plan.Commands {
		if len(args) == 0 || args[0] != "iptables" {
			t.Fatalf("unexpected command: %v", args)
		}
		line := " " + strings.Join(args, " ") + " "
		if strings.Contains(line, " --set ") {
			sets++
		}
		if strings.Contains(line, " --remove ") || strings.Contains(line, " --rremove ") || strings.Contains(line, " --flush ") {
			removes++
		}
		if strings.Contains(line, " -I INPUT ") || strings.Contains(line, " -A INPUT ") || strings.Contains(line, " -I FORWARD ") || strings.Contains(line, " -A FORWARD ") {
			hooks++
		}
		if strings.Contains(line, " -j ACCEPT ") {
			accepts++
		}
	}
	if sets != 3 || removes != 0 {
		t.Fatalf("expected known stale-state gap: sets=%d removes=%d", sets, removes)
	}
	if hooks != 0 || accepts != 0 {
		t.Fatalf("preview must not install hooks or ACCEPT: hooks=%d accepts=%d", hooks, accepts)
	}

	// Desired state machine rejects interleaving and duplicate stage-2. The
	// plan's recent-list stage entries are not revoked when these arrive.
	for _, trace := range [][]int{
		{41001, 41003, 41002, 41003},
		{41001, 41002, 41002, 41003},
	} {
		m := newStrictKnockModel()
		for i, port := range trace {
			m.packet("198.51.100.10", port, i, 30, 120, 2222)
		}
		if m.packet("198.51.100.10", 2222, len(trace), 30, 120, 2222) {
			t.Fatalf("strict model accepted interleaving: %v", trace)
		}
	}
}
