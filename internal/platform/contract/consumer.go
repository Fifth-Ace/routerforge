package contract

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

var ErrConsumer = errors.New("unsupported consumer or group")

type ConsumerGroup struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Type     policy.ObjectType `json:"type"`
	Revision int64             `json:"revision"`
}
type ConsumerPreview struct {
	SchemaVersion int           `json:"schemaVersion"`
	ETag          string        `json:"etag"`
	Group         ConsumerGroup `json:"group"`
	Consumer      string        `json:"consumer"`
	Values        []string      `json:"values"`
	Blocked       []string      `json:"blocked"`
}

func consumerTypes(consumer string, t policy.ObjectType) bool {
	switch consumer {
	case "dns", "nfqws-domain":
		return t == policy.DomainGroup
	case "nfqws-ip":
		return t == policy.Network
	case "network":
		return t == policy.Network || t == policy.DomainGroup || t == policy.Device
	}
	return false
}
func consumerCatalog(r Revision) ([]policy.Group, map[string]any, error) {
	if err := ValidateCatalog(r.Catalog); err != nil {
		return nil, nil, err
	}
	v, err := Decode(r.Catalog)
	if err != nil {
		return nil, nil, err
	}
	c := obj(v)
	b, err := json.Marshal(c["policyObjects"])
	if err != nil {
		return nil, nil, err
	}
	var groups []policy.Group
	err = json.Unmarshal(b, &groups)
	return groups, c, err
}
func ConsumerGroups(r Revision, consumer string) ([]ConsumerGroup, error) {
	if !consumerTypes(consumer, policy.Network) && !consumerTypes(consumer, policy.DomainGroup) {
		return nil, ErrConsumer
	}
	groups, _, err := consumerCatalog(r)
	if err != nil {
		return nil, err
	}
	out := []ConsumerGroup{}
	for _, g := range groups {
		if consumerTypes(consumer, g.Type) {
			out = append(out, ConsumerGroup{g.ID, g.Name, g.Type, g.Revision})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ProjectConsumer is pure: no module I/O, probes, rules, or credential access.
// Unsupported leaves block the whole projection, preventing silent truncation.
func ProjectConsumer(r Revision, consumer, id string) (ConsumerPreview, error) {
	return ProjectConsumerWithNative(r, consumer, id, nil)
}

// The resolver is injected by Core; the original pure projection remains usable.
// Each selected source must match the explicit full SHA256 saved in the catalogue.
func ProjectConsumerWithNative(r Revision, consumer, id string, resolve NativeListResolver) (ConsumerPreview, error) {
	groups, c, err := consumerCatalog(r)
	if err != nil {
		return ConsumerPreview{}, err
	}
	var group *policy.Group
	for i := range groups {
		if groups[i].ID == id {
			group = &groups[i]
			break
		}
	}
	if group == nil || !consumerTypes(consumer, group.Type) {
		return ConsumerPreview{}, ErrConsumer
	}
	p := ConsumerPreview{1, r.ETag, ConsumerGroup{group.ID, group.Name, group.Type, group.Revision}, consumer, []string{}, []string{}}
	members, err := policy.Expand(groups, id)
	if err != nil {
		return p, err
	}
	seen := map[string]bool{}
	blocked := map[string]bool{}
	reject := func(code string) {
		if !blocked[code] {
			blocked[code] = true
			p.Blocked = append(p.Blocked, code)
		}
	}
	add := func(v string) {
		if !seen[v] {
			if len(p.Values) >= policy.MaxExpanded {
				reject("EXPANSION_LIMIT")
				return
			}
			seen[v] = true
			p.Values = append(p.Values, v)
		}
	}
	cache := map[string]NativeList{}
	unavailable := map[string]bool{}
	attempts := 0
	for _, m := range members {
		switch m.Kind {
		case "domain":
			if strings.HasPrefix(m.Value, "*.") {
				reject("WILDCARD_REQUIRES_NATIVE_SEMANTICS")
				continue
			}
			add(m.Value)
		case "ip":
			add(m.Value)
		case "cidr":
			if consumer == "nfqws-ip" {
				add(m.Value)
			} else {
				reject("CIDR_NOT_SINGLE_TARGET")
			}
		case "list-ref":
			ref, ok := policy.ParseNativeListReference(m.Value)
			if !ok || resolve == nil {
				reject("LIST_REFERENCE_REQUIRES_NATIVE_RESOLVER")
				continue
			}
			if ref.SHA256 == "" {
				reject("NATIVE_REFERENCE_UNPINNED")
				continue
			}
			key := ref.Key()
			if unavailable[key] {
				reject("NATIVE_LIST_UNAVAILABLE_OR_UNSUPPORTED")
				continue
			}
			list, found := cache[key]
			if !found {
				if attempts >= NativeReferenceBudget {
					reject("NATIVE_REFERENCE_LIMIT")
					continue
				}
				attempts++
				var e error
				list, e = resolve(ref)
				if e != nil {
					unavailable[key] = true
					reject("NATIVE_LIST_UNAVAILABLE_OR_UNSUPPORTED")
					continue
				}
				cache[key] = list
			}
			if list.SHA256 != ref.SHA256 {
				reject("NATIVE_REFERENCE_STALE")
				continue
			}
			values, e := nativeDomains(list.Values)
			if e != nil {
				reject("NATIVE_LIST_UNAVAILABLE_OR_UNSUPPORTED")
				continue
			}
			for _, v := range values {
				add(v)
			}
		case "device":
			enabled := false
			for _, v := range arr(c["devices"]) {
				d := obj(v)
				if d["id"] == m.Value && d["enabled"] == true {
					enabled = true
					break
				}
			}
			if !enabled {
				reject("DEVICE_DISABLED")
				continue
			}
			found := false
			for _, v := range arr(c["connections"]) {
				co := obj(v)
				if co["deviceId"] != m.Value || co["enabled"] != true {
					continue
				}
				for _, v := range arr(co["endpoints"]) {
					ep := obj(v)
					add(str(ep["host"]))
					found = true
				}
			}
			if !found {
				reject("DEVICE_HAS_NO_ENABLED_ENDPOINT")
			}
		default:
			reject("UNSUPPORTED_MEMBER")
		}
	}
	if len(p.Values) > policy.MaxExpanded {
		reject("EXPANSION_LIMIT")
	}
	if len(p.Blocked) > 0 {
		p.Values = []string{}
	}
	sort.Strings(p.Values)
	sort.Strings(p.Blocked)
	return p, nil
}

// ValidateMetrics checks an unbound local metrics projection against the same v1 schema.
func ValidateMetrics(b []byte) error {
	v, e := Decode(b)
	if e != nil {
		return e
	}
	if e = check(def("metrics"), v, "metrics"); e != nil {
		return e
	}
	m := obj(v)
	pairs := []any{m["memory"], m["swap"]}
	if m["filesystems"] != nil {
		pairs = append(pairs, arr(m["filesystems"])...)
	}
	for _, v := range pairs {
		if v != nil {
			p := obj(v)
			if p["usedBytes"].(float64) > p["totalBytes"].(float64) {
				return errAt("bytes")
			}
			if a, ok := p["availableBytes"].(float64); ok && a > p["totalBytes"].(float64) {
				return errAt("bytes")
			}
		}
	}
	return nil
}
