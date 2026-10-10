package main

import (
	"context"
	"testing"
	"time"
)

func TestK4CanonicalPlanAndRejection(t *testing.T) {
	p, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	if err := validateCanonicalIsolatedPlan(p.Commands); err != nil {
		t.Fatalf("canonical rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func([][]string)
	}{
		{"extra-jump", func(c [][]string) { c[3] = append(c[3], "-j", "DROP") }},
		{"extra-match", func(c [][]string) { c[3] = append(c[3], "-m", "comment", "--comment", "unexpected") }},
		{"wrong-step", func(c [][]string) { c[4][len(c[4])-1] = "RF_KNOCK_STEP3" }},
		{"changed-target", func(c [][]string) {
			for i, x := range c[6] {
				if x == "--dport" {
					c[6][i+1] = "2223"
					break
				}
			}
		}},
		{"wrong-window", func(c [][]string) {
			for i, x := range c[5] {
				if x == "--seconds" {
					c[5][i+1] = "29"
					break
				}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			clone := make([][]string, len(p.Commands))
			for i := range p.Commands {
				clone[i] = append([]string(nil), p.Commands[i]...)
			}
			tc.edit(clone)
			if err := validateCanonicalIsolatedPlan(clone); err == nil {
				t.Fatal("unsafe plan accepted")
			}
			hostile := p
			hostile.Commands = clone
			r := &recordedRunner{}
			if err := (isolatedExecutor{runner: r, timeout: time.Second}).exercise(context.Background(), hostile, func(context.Context) error { return nil }); err == nil || len(r.calls) != 0 {
				t.Fatalf("executor ran unsafe plan: %v calls=%v", err, r.calls)
			}
		})
	}
}
