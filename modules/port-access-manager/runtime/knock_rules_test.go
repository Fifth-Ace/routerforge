package main

import (
	"reflect"
	"testing"
)

func TestKnockPlanDryRun(t *testing.T) {
	o := KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120}
	plan, err := buildKnockRules(o)
	if err != nil || plan.Hooked || plan.Applied || len(plan.Commands) != 6 {
		t.Fatalf("unsafe plan: %+v: %v", plan, err)
	}
	if !reflect.DeepEqual(plan.Commands[0], []string{"iptables", "-t", "filter", "-N", "RF_PORT_KNOCK"}) {
		t.Fatal(plan.Commands[0])
	}
	for _, cmd := range plan.Commands {
		if len(cmd) < 5 || cmd[0] != "iptables" {
			t.Fatalf("unexpected command: %v", cmd)
		}
	}
}
func TestKnockPlanRejectsBadInput(t *testing.T) {
	good := KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120}
	candidates := []KnockOptions{}
	a := good
	a.Target = 22
	a.Sequence[1] = 22
	candidates = append(candidates, a)
	a = good
	a.Sequence[2] = 65536
	candidates = append(candidates, a)
	a = good
	a.WindowSeconds = 0
	candidates = append(candidates, a)
	a = good
	a.AccessSeconds = 3601
	candidates = append(candidates, a)
	for _, o := range candidates {
		if _, err := buildKnockRules(o); err == nil {
			t.Fatalf("accepted invalid: %+v", o)
		}
	}
}
