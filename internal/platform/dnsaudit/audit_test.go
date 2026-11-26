package dnsaudit

import (
	"fmt"
	"testing"
)

func TestRankPrecedence(t *testing.T) {
	rules := []Rule{{ID: "parent", Domain: "example.org", Match: "suffix", Priority: 0}, {ID: "sub", Domain: "a.example.org", Match: "suffix", Priority: 0}, {ID: "z", Domain: "a.example.org", Match: "exact", Priority: 1}, {ID: "a", Domain: "A.example.org.", Match: "exact", Priority: 1}, {ID: "late", Domain: "a.example.org", Match: "exact", Priority: 2}, {ID: "other", Domain: "notexample.org", Match: "suffix"}, {ID: "children", Domain: "a.example.org", Match: "subdomains"}}
	r := Rank("A.example.org.", rules)
	if r.Total != 5 {
		t.Fatal(r)
	}
	want := []string{"a", "z", "late", "sub", "parent"}
	reasons := []string{"WINNER", "RULE_ID_TIEBREAK", "PRIORITY", "MATCH_RANK", "LESS_SPECIFIC"}
	for i, x := range r.Rows {
		if x.ID != want[i] || x.Reason != reasons[i] {
			t.Fatal(r)
		}
	}
}
func TestRankLabelBoundary(t *testing.T) {
	r := []Rule{{ID: "a", Domain: "example.org", Match: "subdomains"}}
	for _, d := range []string{"example.org", "badexample.org", "example.org.net"} {
		if Rank(d, r).Total != 0 {
			t.Fatal(d)
		}
	}
	if Rank("x.example.org", r).Total != 1 {
		t.Fatal("subdomains")
	}
}
func TestRankBound(t *testing.T) {
	r := []Rule{}
	for i := 0; i < 5000; i++ {
		r = append(r, Rule{ID: fmt.Sprintf("r%05d", i), Domain: "example.org", Match: "suffix", Priority: 5000 - i})
	}
	x := Rank("www.example.org", r)
	if x.Total != 5000 || len(x.Rows) != MaxCandidates || x.Rows[0].ID != "r04999" {
		t.Fatal(x)
	}
}
func TestConflictingTargets(t *testing.T) {
	r := []Rule{{ID: "z", Domain: "a.example.org", Match: "suffix", Priority: 1, Upstream: "a"}, {ID: "a", Domain: "A.example.org.", Match: "suffix", Priority: 1, Upstream: "b"}, {ID: "b", Domain: "a.example.org", Match: "suffix", Priority: 1, Upstream: "a"}, {ID: "other", Domain: "a.example.org", Match: "exact", Priority: 1, Upstream: "c"}, {ID: "same", Domain: "a.example.org", Match: "suffix", Priority: 2, Upstream: "b"}}
	x := Audit(r)
	if x.Total != 1 || x.Rows[0].WinnerID != "a" || x.Rows[0].RuleCount != 3 {
		t.Fatal(x)
	}
}
func TestNoFalseConflict(t *testing.T) {
	r := []Rule{{ID: "a", Domain: "example.org", Match: "suffix", Upstream: "a"}, {ID: "b", Domain: "example.org", Match: "suffix", Upstream: "a"}}
	if Audit(r).Total != 0 {
		t.Fatal(r)
	}
}
func TestConflictBound(t *testing.T) {
	r := []Rule{}
	for i := 0; i < 80; i++ {
		for _, u := range []string{"a", "b"} {
			r = append(r, Rule{ID: fmt.Sprintf("r%d%s", i, u), Domain: fmt.Sprintf("d%03d.example.org", i), Match: "suffix", Upstream: u})
		}
	}
	x := Audit(r)
	if x.Total != 80 || len(x.Rows) != MaxConflicts || x.Rows[0].Domain != "d000.example.org" {
		t.Fatal(x)
	}
}
