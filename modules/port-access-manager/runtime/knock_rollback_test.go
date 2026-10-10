package main

import (
	"reflect"
	"testing"
)

func TestKnockRollbackOwnChainOnly(t *testing.T) {
	got := buildKnockRollback()
	want := [][]string{
		{"iptables", "-t", "filter", "-F", "RF_PORT_KNOCK"},
		{"iptables", "-t", "filter", "-X", "RF_PORT_KNOCK"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rollback changed: %v", got)
	}
}

func TestKnockPlanOrderAndNoHooks(t *testing.T) {
	plan, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 12345, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil || plan.Applied || plan.Hooked || len(plan.Commands) != 6 {
		t.Fatalf("unsafe plan: %+v: %v", plan, err)
	}
	if !reflect.DeepEqual(plan.Commands[0], []string{"iptables", "-t", "filter", "-N", "RF_PORT_KNOCK"}) {
		t.Fatal("unexpected chain create", plan.Commands[0])
	}
	for i, port := range []string{"41001", "41002", "41003", "12345", "12345"} {
		args := plan.Commands[i+1]
		found := false
		for j := 0; j+1 < len(args); j++ {
			if args[j] == "--dport" && args[j+1] == port {
				found = true
			}
			if args[j] == "-I" || args[j] == "_NDM_FORWARD" || args[j] == "FORWARD" || args[j] == "INPUT" {
				t.Fatalf("hook in dry-run: %v", args)
			}
		}
		if !found {
			t.Fatalf("rule %d wrong target: %v", i, args)
		}
	}
	last := plan.Commands[5]
	if last[len(last)-1] != "DROP" {
		t.Fatalf("no terminal DROP: %v", last)
	}
}
