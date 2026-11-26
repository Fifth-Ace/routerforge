// Package dnsaudit contains diagnostic-only ranking and bounded list summaries.
// It does not participate in DNS forwarding or alter the policy engine.
package dnsaudit

import (
	"sort"
	"strings"
)

const MaxCandidates = 32
const MaxConflicts = 32

type Rule struct {
	ID       string `json:"id"`
	Domain   string `json:"domain"`
	Match    string `json:"match"`
	Priority int    `json:"priority"`
	Upstream string `json:"upstream"`
	GroupID  string `json:"group_id,omitempty"`
	ListID   string `json:"list_id,omitempty"`
}

type Candidate struct {
	Rule
	Reason string `json:"reason"`
}
type Ranking struct {
	Total int         `json:"total"`
	Rows  []Candidate `json:"rows"`
}

func canonical(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}
func depth(s string) int { return strings.Count(s, ".") + 1 }
func rank(s string) int {
	if s == "exact" {
		return 2
	}
	if s == "suffix" {
		return 1
	}
	return 0
}
func less(a, b Rule) bool {
	if depth(a.Domain) != depth(b.Domain) {
		return depth(a.Domain) > depth(b.Domain)
	}
	if rank(a.Match) != rank(b.Match) {
		return rank(a.Match) > rank(b.Match)
	}
	if a.Priority != b.Priority {
		return a.Priority < b.Priority
	}
	return a.ID < b.ID
}
func why(a, b Rule) string {
	if depth(a.Domain) != depth(b.Domain) {
		return "LESS_SPECIFIC"
	}
	if rank(a.Match) != rank(b.Match) {
		return "MATCH_RANK"
	}
	if a.Priority != b.Priority {
		return "PRIORITY"
	}
	return "RULE_ID_TIEBREAK"
}

// Rank accepts already validated rules. For each matching rule, explain the
// precedence against the winner. Caller verifies winner against native Evaluate.
func Rank(domain string, rules []Rule) Ranking {
	d := canonical(domain)
	matches := make([]Rule, 0)
	for _, r := range rules {
		r.Domain = canonical(r.Domain)
		exact := d == r.Domain
		child := strings.HasSuffix(d, "."+r.Domain)
		if (r.Match == "exact" && exact) || (r.Match == "suffix" && (exact || child)) || (r.Match == "subdomains" && child) {
			matches = append(matches, r)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return less(matches[i], matches[j]) })
	out := Ranking{Total: len(matches), Rows: []Candidate{}}
	for i, r := range matches {
		if i >= MaxCandidates {
			break
		}
		reason := "WINNER"
		if i > 0 {
			reason = why(matches[0], r)
		}
		out.Rows = append(out.Rows, Candidate{r, reason})
	}
	return out
}

type Conflict struct {
	Domain    string `json:"domain"`
	Match     string `json:"match"`
	Priority  int    `json:"priority"`
	WinnerID  string `json:"winner_id"`
	RuleCount int    `json:"rule_count"`
}
type Conflicts struct {
	Total int        `json:"total"`
	Rows  []Conflict `json:"rows"`
}

// Conflicting equal-precedence targets in the same policy layer. This is not
// a whole-network reachability analysis or a proof of unreachable rules.
func Audit(rules []Rule) Conflicts {
	type key struct {
		domain, match string
		priority      int
	}
	type group struct {
		winner   Rule
		count    int
		conflict bool
	}
	groups := make(map[key]group)
	for _, r := range rules {
		r.Domain = canonical(r.Domain)
		k := key{r.Domain, r.Match, r.Priority}
		g, ok := groups[k]
		if !ok {
			g = group{winner: r}
		} else {
			if g.winner.Upstream != r.Upstream || g.winner.GroupID != r.GroupID {
				g.conflict = true
			}
			if r.ID < g.winner.ID {
				g.winner = r
			}
		}
		g.count++
		groups[k] = g
	}
	rows := []Conflict{}
	for k, g := range groups {
		if g.conflict {
			rows = append(rows, Conflict{k.domain, k.match, k.priority, g.winner.ID, g.count})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Domain != b.Domain {
			return a.Domain < b.Domain
		}
		if a.Match != b.Match {
			return a.Match < b.Match
		}
		return a.Priority < b.Priority
	})
	n := len(rows)
	if n > MaxConflicts {
		rows = rows[:MaxConflicts]
	}
	return Conflicts{n, rows}
}
