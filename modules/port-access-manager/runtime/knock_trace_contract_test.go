package main

import "testing"

// K4Q is a model-only trace matrix. It does not claim that the generated
// iptables plan is equivalent to this model or authorize kernel deployment.
func TestK4QTraceMatrix(t *testing.T) {
	const ip = "198.51.100.10"
	const target = 2222
	cases := []struct {
		name string
		ports []int
		allow bool
	}{
		{"ordered", []int{41001, 41002, 41003}, true},
		{"missing-first", []int{41002, 41003}, false},
		{"missing-second", []int{41001, 41003}, false},
		{"out-of-order", []int{41003, 41002, 41001}, false},
		{"duplicate-first", []int{41001, 41001, 41002, 41003}, true},
		{"duplicate-second", []int{41001, 41002, 41002, 41003}, false},
		{"duplicate-third", []int{41001, 41002, 41003, 41003}, false},
		{"interleave", []int{41001, 41003, 41002, 41003}, false},
		{"restart-incomplete", []int{41001, 41002, 41001, 41003}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newStrictKnockModel()
			for i, port := range tc.ports {
				if m.packet(ip, port, i, 30, 120, target) {
					t.Fatalf("knock port %d granted access", port)
				}
			}
			if got := m.packet(ip, target, len(tc.ports), 30, 120, target); got != tc.allow {
				t.Fatalf("authorized=%t, want %t, trace=%v", got, tc.allow, tc.ports)
			}
		})
	}
}

func TestK4QSourceIsolationAndRevocation(t *testing.T) {
	m := newStrictKnockModel()
	const ip = "198.51.100.10"
	const other = "203.0.113.25"
	for i, port := range []int{41001, 41002, 41003} {
		m.packet(ip, port, i, 30, 120, 2222)
	}
	if m.packet(other, 2222, 3, 30, 120, 2222) {
		t.Fatal("another source inherited authorization")
	}
	if !m.packet(ip, 2222, 122, 30, 120, 2222) {
		t.Fatal("grant should remain valid at TTL boundary")
	}
	if m.packet(ip, 2222, 123, 30, 120, 2222) {
		t.Fatal("grant survives past TTL")
	}
	// A fresh first knock must revoke a prior grant until a full new sequence.
	m = newStrictKnockModel()
	for i, port := range []int{41001, 41002, 41003} {
		m.packet(ip, port, i, 30, 120, 2222)
	}
	m.packet(ip, 41001, 4, 30, 120, 2222)
	if m.packet(ip, 2222, 5, 30, 120, 2222) {
		t.Fatal("restart retained prior authorization")
	}
}

func TestK4QWindowBoundary(t *testing.T) {
	const ip = "198.51.100.10"
	for _, tc := range []struct {
		name string
		second int
		third int
		allowed bool
	}{
		{"inside", 15, 29, true},
		{"exact-window", 15, 30, true},
		{"past-window", 15, 31, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newStrictKnockModel()
			m.packet(ip, 41001, 0, 30, 120, 2222)
			m.packet(ip, 41002, tc.second, 30, 120, 2222)
			m.packet(ip, 41003, tc.third, 30, 120, 2222)
			if got := m.packet(ip, 2222, tc.third+1, 30, 120, 2222); got != tc.allowed {
				t.Fatalf("authorization=%t want=%t", got, tc.allowed)
			}
		})
	}
}

func TestK4QDeploymentRemainsBlocked(t *testing.T) {
	plan, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Hooked || plan.Applied || plan.StrictSequenceVerified || !plan.DeploymentBlocked || len(plan.DeploymentBlockers) == 0 {
		t.Fatalf("unsafe deployment state: %+v", plan)
	}
}
