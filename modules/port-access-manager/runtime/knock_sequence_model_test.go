package main

import (
	"testing"
)

// Model of the current dry-run recent rules, not a kernel/iptables integration test.
// A state entry is set per originating source address; timeout is measured from set.
type knockTraceState struct{ stage1, stage2, authorized map[string]int }

func newKnockTraceState() knockTraceState {
	return knockTraceState{map[string]int{}, map[string]int{}, map[string]int{}}
}
func (s knockTraceState) packet(src string, port, second, now, window, ttl int) bool {
	switch port {
	case 41001:
		s.stage1[src] = now
	case 41002:
		if when, ok := s.stage1[src]; ok && now-when <= window {
			s.stage2[src] = now
		}
	case 41003:
		if when, ok := s.stage2[src]; ok && now-when <= window {
			s.authorized[src] = now
		}
	default:
		if port == second {
			when, ok := s.authorized[src]
			return ok && now-when <= ttl
		}
	}
	return false
}
func TestK4MCurrentPlanOutOfOrderTrace(t *testing.T) {
	s := newKnockTraceState()
	for _, step := range []struct{ p, at int }{{41001, 0}, {41002, 1}, {41001, 2}, {41003, 3}} {
		s.packet("198.51.100.10", step.p, 2222, step.at, 30, 120)
	}
	// K4L's step1 rule does not remove stage2; the next step3 succeeds.
	if !s.packet("198.51.100.10", 2222, 2222, 4, 30, 120) {
		t.Fatal("model drift: current plan no longer accepts 1,2,1,3; revisit strictness claim")
	}
	t.Log("KNOWN UNSAFE: 1,2,1,3 grants access in current K4L model; real apply remains blocked")
}
func TestK4MTimeoutAndSourceSeparation(t *testing.T) {
	s := newKnockTraceState()
	s.packet("198.51.100.10", 41001, 2222, 0, 30, 120)
	s.packet("198.51.100.10", 41002, 2222, 31, 30, 120)
	s.packet("198.51.100.10", 41003, 2222, 32, 30, 120)
	if s.packet("198.51.100.10", 2222, 2222, 33, 30, 120) {
		t.Fatal("expired first step granted access")
	}
	s = newKnockTraceState()
	s.packet("198.51.100.10", 41001, 2222, 0, 30, 120)
	s.packet("198.51.100.10", 41002, 2222, 1, 30, 120)
	s.packet("198.51.100.10", 41003, 2222, 2, 30, 120)
	if s.packet("203.0.113.55", 2222, 2222, 3, 30, 120) {
		t.Fatal("authorization leaked to another source")
	}
	if !s.packet("198.51.100.10", 2222, 2222, 3, 30, 120) {
		t.Fatal("valid sequence failed in model")
	}
	if s.packet("198.51.100.10", 2222, 2222, 123, 30, 120) {
		t.Fatal("authorization outlived ttl")
	}
}
func TestK4MPlanRemainsUnhooked(t *testing.T) {
	plan, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil || plan.Applied || plan.Hooked {
		t.Fatalf("unsafe plan state %+v: %v", plan, err)
	}
}
