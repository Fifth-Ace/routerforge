package policy

import "testing"

func TestGroupsExpansion(t *testing.T) {
	a := Group{ID: "net-a", Type: Network, Name: "a", Revision: 1, Scope: "local", Members: []Member{{Kind: "cidr", Value: "192.0.2.0/24"}}}
	b := Group{ID: "net-b", Type: Network, Name: "b", Revision: 1, Scope: "local", Members: []Member{{Kind: "object", Value: "net-a"}, {Kind: "ip", Value: "2001:db8::1"}}}
	groups := []Group{a, b}
	if e := ValidateGroups(groups, nil); e != nil {
		t.Fatal(e)
	}
	x, e := Expand(groups, "net-b")
	if e != nil || len(x) != 2 {
		t.Fatal(x, e)
	}
	if e = ValidateGroups([]Group{b}, nil); e == nil {
		t.Fatal("deleted referenced group accepted")
	}
	groups[0].Members = append(groups[0].Members, Member{Kind: "object", Value: "net-b"})
	if e = ValidateGroups(groups, nil); e == nil {
		t.Fatal("cycle accepted")
	}
}
