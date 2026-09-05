package main

import "testing"

func TestParseIPPoliciesMetadata(t *testing.T) {
	input := `
policy, name = Policy0, description = 123:
  mark: ffffaaa
  table4: 4096
  route4:
    route:
      destination: 192.168.1.0/24

policy, name = Policy1, description = nfqws:
  mark: ffffaab
  table4: 4098
  route4:
    route:
      destination: 0.0.0.0/0
      gateway: 192.168.1.1
`
	got := parseIPPolicies(input)

	p0 := got["Policy0"]
	if p0.Description != "123" || p0.HasDefault {
		t.Fatalf("Policy0=%#v", p0)
	}
	p1 := got["Policy1"]
	if p1.Description != "nfqws" || !p1.HasDefault {
		t.Fatalf("Policy1=%#v", p1)
	}
}
