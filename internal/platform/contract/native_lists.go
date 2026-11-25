package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

const NativeListLimit = 4096
const NativeReferenceBudget = 8
const NativePayloadLimit = 2 << 20

var ErrNativeList = errors.New("native list unavailable or unsupported")

type NativeList struct {
	SHA256 string   `json:"sha256"`
	Values []string `json:"-"`
}
type NativeListResolver func(policy.NativeListReference) (NativeList, error)
type NativeReferenceUse struct {
	Ref    string   `json:"ref"`
	Groups []string `json:"groups"`
}

// Direct and transitive catalogue references, not claims about applied rules.
func NativeReferenceIndex(r Revision) ([]NativeReferenceUse, error) {
	groups, _, e := consumerCatalog(r)
	if e != nil {
		return nil, e
	}
	refs := map[string][]string{}
	for _, g := range groups {
		members, e := policy.Expand(groups, g.ID)
		if e != nil {
			return nil, e
		}
		for _, m := range members {
			if m.Kind == "list-ref" {
				refs[m.Value] = append(refs[m.Value], g.ID)
			}
		}
	}
	out := []NativeReferenceUse{}
	for ref, ids := range refs {
		sort.Strings(ids)
		out = append(out, NativeReferenceUse{ref, ids})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Ref < out[j].Ref })
	return out, nil
}
func nativeDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func nativeDomains(raw []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range raw {
		if _, err := netip.ParseAddr(v); err == nil {
			return nil, ErrNativeList
		}
		if !policy.ValidDomain(v) || strings.HasSuffix(v, ".") {
			return nil, ErrNativeList
		}
		v = strings.ToLower(v)
		if !seen[v] {
			if len(out) >= NativeListLimit {
				return nil, ErrNativeList
			}
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out, nil
}

// Extract only the selected managed list from DNS Router's saved draft. The
// fingerprint covers selected list metadata and rules, not unrelated upstreams.
// URLs, upstream configuration and other raw fields are never returned.
func DNSNativeList(raw []byte, name string) (NativeList, error) {
	if _, ok := policy.ParseNativeListReference("dns." + name); !ok {
		return NativeList{}, ErrNativeList
	}
	if _, e := Decode(raw); e != nil {
		return NativeList{}, ErrNativeList
	}
	var payload struct {
		Revision string `json:"revision"`
		Config   struct {
			SchemaVersion int `json:"schema_version"`
			Lists         []struct {
				ID       string `json:"id"`
				Match    string `json:"match"`
				Upstream string `json:"upstream"`
				GroupID  string `json:"group_id"`
				Priority int    `json:"priority"`
			} `json:"lists"`
			Rules []struct {
				ID       string `json:"id"`
				ListID   string `json:"list_id"`
				Domain   string `json:"domain"`
				Match    string `json:"match"`
				Upstream string `json:"upstream"`
				GroupID  string `json:"group_id"`
				Priority int    `json:"priority"`
			} `json:"rules"`
		} `json:"config"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.Revision == "" || payload.Config.SchemaVersion != 1 {
		return NativeList{}, ErrNativeList
	}
	idx := -1
	for i, l := range payload.Config.Lists {
		if l.ID == name {
			if idx != -1 {
				return NativeList{}, ErrNativeList
			}
			idx = i
		}
	}
	if idx < 0 {
		return NativeList{}, ErrNativeList
	}
	l := payload.Config.Lists[idx]
	if l.Match != "suffix" && l.Match != "exact" && l.Match != "subdomains" {
		return NativeList{}, ErrNativeList
	}
	rawValues := []string{}
	for _, r := range payload.Config.Rules {
		if r.ListID != name {
			continue
		}
		if r.Match != l.Match || r.Upstream != l.Upstream || r.GroupID != l.GroupID || r.Priority != l.Priority || len(rawValues) >= NativeListLimit {
			return NativeList{}, ErrNativeList
		}
		rawValues = append(rawValues, r.Domain)
	}
	values, e := nativeDomains(rawValues)
	if e != nil {
		return NativeList{}, e
	}
	data, e := json.Marshal(struct {
		List   any
		Values []string
	}{l, values})
	if e != nil {
		return NativeList{}, e
	}
	return NativeList{nativeDigest(data), values}, nil
}

// NFQWS native API returns exact file bytes as UTF-8 text and its SHA256.
// Only plain domain lines and comments are supported. IP/CIDR, wildcard and
// special expressions are rejected as a whole instead of silently filtered.
func NFQWSNativeList(raw []byte, name string) (NativeList, error) {
	if _, ok := policy.ParseNativeListReference("nfqws." + name); !ok {
		return NativeList{}, ErrNativeList
	}
	if _, e := Decode(raw); e != nil {
		return NativeList{}, ErrNativeList
	}
	var p struct {
		Name    string  `json:"name"`
		Content *string `json:"content"`
		SHA256  string  `json:"sha256"`
		Size    *int    `json:"size"`
	}
	if json.Unmarshal(raw, &p) != nil || p.Name != name || p.Content == nil || p.Size == nil || *p.Size != len(*p.Content) || !utf8.ValidString(*p.Content) || nativeDigest([]byte(*p.Content)) != p.SHA256 {
		return NativeList{}, ErrNativeList
	}
	values := []string{}
	for _, line := range strings.Split(*p.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(values) >= NativeListLimit {
			return NativeList{}, ErrNativeList
		}
		values = append(values, line)
	}
	out, e := nativeDomains(values)
	if e != nil {
		return NativeList{}, e
	}
	return NativeList{p.SHA256, out}, nil
}
