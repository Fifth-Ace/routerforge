package policy

import (
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// Group extends the existing policy object contract without changing legacy objects.
// All members are declarative; this package never applies network rules.
type Member struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type Group struct {
	ID          string     `json:"id"`
	Type        ObjectType `json:"type"`
	Name        string     `json:"name"`
	Revision    int64      `json:"revision"`
	Scope       string     `json:"scope"`
	Members     []Member   `json:"members"`
	Description string     `json:"description,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
}

const MaxGroups = 100
const MaxExpanded = 4096

var groupID = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,96}$`)
var domainLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func ValidDomain(s string) bool {
	s = strings.TrimSuffix(s, ".")
	if len(s) == 0 || len(s) > 253 {
		return false
	}
	for _, p := range strings.Split(s, ".") {
		if !domainLabel.MatchString(p) {
			return false
		}
	}
	return true
}
func ValidHost(s string) bool {
	a, e := netip.ParseAddr(s)
	return (e == nil && a.Zone() == "") || ValidDomain(s)
}
func validPort(s string) bool {
	parts := strings.Split(s, ":")
	if len(parts) != 2 || (parts[0] != "tcp" && parts[0] != "udp") {
		return false
	}
	v := strings.Split(parts[1], "-")
	if len(v) > 2 {
		return false
	}
	lo, e := strconv.Atoi(v[0])
	if e != nil || lo < 1 || lo > 65535 || strconv.Itoa(lo) != v[0] {
		return false
	}
	if len(v) == 1 {
		return true
	}
	hi, e := strconv.Atoi(v[1])
	return e == nil && hi >= lo && hi <= 65535 && strconv.Itoa(hi) == v[1]
}
func validateMember(t ObjectType, m Member, devices map[string]bool) bool {
	if m.Value == "" || len(m.Value) > 253 {
		return false
	}
	switch m.Kind {
	case "object":
		return groupID.MatchString(m.Value)
	case "device":
		return t == Device && devices[m.Value]
	case "ip":
		a, e := netip.ParseAddr(m.Value)
		return t == Network && e == nil && a.Zone() == ""
	case "cidr":
		p, e := netip.ParsePrefix(m.Value)
		return t == Network && e == nil && p == p.Masked()
	case "domain":
		return t == DomainGroup && ValidDomain(strings.TrimPrefix(m.Value, "*."))
	case "list-ref":
		return t == DomainGroup && groupID.MatchString(m.Value)
	case "port":
		return t == PortGroup && validPort(m.Value)
	}
	return false
}

// ValidateGroups rejects dangling or incompatible refs and bounds expansion.
func ValidateGroups(groups []Group, devices map[string]bool) error {
	if len(groups) > MaxGroups {
		return fmt.Errorf("policy group limit")
	}
	byID := map[string]Group{}
	for _, g := range groups {
		if !groupID.MatchString(g.ID) || g.Revision < 1 || g.Revision > 9007199254740991 || (g.Scope != "local" && g.Scope != "shared") || g.Members == nil || len(g.Members) > 1024 {
			return fmt.Errorf("invalid policy group")
		}
		if err := (Object{ID: g.ID, Type: g.Type, Name: g.Name}).Validate(); err != nil {
			return fmt.Errorf("invalid policy object")
		}
		if len(g.Description) > 2048 || len(g.Tags) > 32 {
			return fmt.Errorf("policy metadata limit")
		}
		if _, ok := byID[g.ID]; ok {
			return fmt.Errorf("duplicate policy object")
		}
		byID[g.ID] = g
		seen := map[Member]bool{}
		for _, m := range g.Members {
			if seen[m] || !validateMember(g.Type, m, devices) {
				return fmt.Errorf("invalid or duplicate policy member")
			}
			seen[m] = true
		}
	}
	for _, g := range groups {
		if _, err := Expand(groups, g.ID); err != nil {
			return err
		}
	}
	return nil
}

// Expand returns deduplicated declarative leaves. list-ref is intentionally not resolved
// until its consumer adapter is available. It must not be mistaken for an applied rule.
func Expand(groups []Group, id string) ([]Member, error) {
	byID := map[string]Group{}
	for _, g := range groups {
		byID[g.ID] = g
	}
	active := map[string]bool{}
	done := map[string]bool{}
	seen := map[Member]bool{}
	out := []Member{}
	work := 0
	var walk func(string, ObjectType) error
	walk = func(key string, t ObjectType) error {
		g, ok := byID[key]
		if !ok || g.Type != t {
			return fmt.Errorf("unknown or incompatible policy reference")
		}
		if active[key] {
			return fmt.Errorf("policy reference cycle")
		}
		if done[key] {
			return nil
		}
		active[key] = true
		for _, m := range g.Members {
			work++
			if work > MaxGroups*1024 {
				return fmt.Errorf("policy work limit")
			}
			if m.Kind == "object" {
				if e := walk(m.Value, t); e != nil {
					return e
				}
			} else if !seen[m] {
				if len(out) >= MaxExpanded {
					return fmt.Errorf("policy expansion limit")
				}
				seen[m] = true
				out = append(out, m)
			}
		}
		delete(active, key)
		done[key] = true
		return nil
	}
	g, ok := byID[id]
	if !ok {
		return nil, fmt.Errorf("unknown policy object")
	}
	if e := walk(id, g.Type); e != nil {
		return nil, e
	}
	return out, nil
}
