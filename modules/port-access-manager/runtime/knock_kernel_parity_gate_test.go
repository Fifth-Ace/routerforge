package main

import (
	"strings"
	"testing"
)

// K4T documents an unsafe gap between the pure strict model and the
// currently generated iptables-recent preview. This is NOT kernel validation.
// A passing test proves that deployment MUST remain blocked.
func TestK4TPreviewDoesNotProveStrictSequence(t *testing.T) {
	options := KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120}
	plan, err := buildKnockRules(options)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hooked || plan.Applied || plan.StrictSequenceVerified || !plan.DeploymentBlocked {
		t.Fatalf("unproven preview must be blocked: %+v", plan)
	}

	// Strict model revokes an old grant when the first knock restarts the
	// sequence. The current iptables recent preview does not contain any
	// action that removes RF_KNOCK_AUTH on the first knock.
	model := newStrictKnockModel()
	const source = "198.51.100.10"
	for i, port := range options.Sequence {
		model.packet(source, port, i, 30, 120, options.Target)
	}
	if !model.packet(source, options.Target, 3, 30, 120, options.Target) {
		t.Fatal("baseline grant unexpectedly absent")
	}
	model.packet(source, options.Sequence[0], 4, 30, 120, options.Target)
	if model.packet(source, options.Target, 5, 30, 120, options.Target) {
		t.Fatal("strict model retained grant after restart")
	}

	first := ""
	for _, cmd := range plan.Commands {
		line := " " + strings.Join(cmd, " ") + " "
		if strings.Contains(line, " --dport 41001 ") {
			first = line
		}
		if strings.Contains(line, " --name RF_KNOCK_AUTH ") && strings.Contains(line, " --remove ") {
			t.Fatal("preview changed: reassess K4T negative parity conclusion")
		}
	}
	if first == "" || !strings.Contains(first, " --name RF_KNOCK_1 ") || !strings.Contains(first, " --set ") {
		t.Fatal("first knock rule changed: reassess negative parity conclusion")
	}
	if strings.Contains(first, " RF_KNOCK_AUTH ") {
		t.Fatal("first knock rule changed: reassess negative parity conclusion")
	}
	if len(plan.DeploymentBlockers) == 0 {
		t.Fatal("negative parity requires explicit deployment blockers")
	}
}
