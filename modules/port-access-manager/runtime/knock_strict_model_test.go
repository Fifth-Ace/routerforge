package main

import "testing"

func TestK4NStrictSequence(t *testing.T) {
	const ip = "198.51.100.10"
	for _, tc := range []struct {
		name  string
		ports []int
		allow bool
	}{
		{"valid", []int{41001, 41002, 41003}, true},
		{"restarted", []int{41001, 41002, 41001, 41003}, false},
		{"wrong-order", []int{41002, 41001, 41003}, false},
		{"repeated-second", []int{41001, 41002, 41002, 41003}, false},
		{"repeated-third", []int{41001, 41003, 41002}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newStrictKnockModel()
			for i, p := range tc.ports {
				m.packet(ip, p, i, 30, 120, 2222)
			}
			if got := m.packet(ip, 2222, len(tc.ports), 30, 120, 2222); got != tc.allow {
				t.Fatalf("got=%v want=%v", got, tc.allow)
			}
		})
	}
}

func TestK4NStrictTimeoutAndIsolation(t *testing.T) {
	m := newStrictKnockModel()
	ip := "198.51.100.10"
	m.packet(ip, 41001, 0, 30, 120, 2222)
	m.packet(ip, 41002, 31, 30, 120, 2222)
	m.packet(ip, 41003, 32, 30, 120, 2222)
	if m.packet(ip, 2222, 33, 30, 120, 2222) {
		t.Fatal("expired step")
	}
	m = newStrictKnockModel()
	m.packet(ip, 41001, 0, 30, 120, 2222)
	m.packet(ip, 41002, 1, 30, 120, 2222)
	m.packet(ip, 41003, 2, 30, 120, 2222)
	if m.packet("203.0.113.55", 2222, 3, 30, 120, 2222) {
		t.Fatal("cross-source access")
	}
	if !m.packet(ip, 2222, 3, 30, 120, 2222) {
		t.Fatal("expected access")
	}
	if m.packet(ip, 2222, 123, 30, 120, 2222) {
		t.Fatal("expired grant")
	}
}

func TestK4NCurrentRulesNotAuthorizedForDeployment(t *testing.T) {
	plan, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil || plan.Hooked || plan.Applied {
		t.Fatalf("unexpected live plan: %+v %v", plan, err)
	}
}
