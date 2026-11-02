package dnsrouter

import (
	"fmt"
	"net"
	"strings"
	"unicode/utf8"
)

// Advanced is optional: omission preserves schema-1 canonical bytes/revisions.
// UI display mode is deliberately not part of routing configuration.
type AdvancedDNS struct {
	Devices []DevicePolicy      `json:"devices,omitempty"`
	Groups  []DevicePolicyGroup `json:"groups,omitempty"`
	Rules   []DeviceDomainRule  `json:"rules,omitempty"`
}
type DeviceTarget struct {
	Upstream string `json:"upstream"`
	GroupID  string `json:"group_id,omitempty"`
}
type DevicePolicy struct {
	MAC            string        `json:"mac"`
	Name           string        `json:"name,omitempty"`
	Target         *DeviceTarget `json:"target,omitempty"`
	InheritDomains bool          `json:"inherit_domains,omitempty"`
}
type DevicePolicyGroup struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	MACs           []string      `json:"macs"`
	Target         *DeviceTarget `json:"target,omitempty"`
	InheritDomains bool          `json:"inherit_domains,omitempty"`
}
type DeviceDomainRule struct {
	ID       string       `json:"id"`
	MAC      string       `json:"mac"`
	Domain   string       `json:"domain"`
	Match    string       `json:"match"`
	Priority int          `json:"priority"`
	Target   DeviceTarget `json:"target"`
}
type ClientDecision struct {
	Decision
	Layer       string `json:"layer"`
	DeviceMAC   string `json:"device_mac,omitempty"`
	DeviceGroup string `json:"device_group,omitempty"`
	Inherited   bool   `json:"inherited_domains,omitempty"`
}
type devicePolicies struct {
	devices map[string]DevicePolicy
	groups  map[string]DevicePolicyGroup
	rules   map[string]*Engine
	active  bool
	targets []Decision
}

func CanonicalDeviceMAC(raw string) (string, error) {
	m, err := net.ParseMAC(raw)
	if err != nil || len(m) != 6 || m[0]&1 != 0 || m.String() == "00:00:00:00:00:00" {
		return "", fmt.Errorf("device requires a nonzero unicast MAC")
	}
	return strings.ToLower(m.String()), nil
}
func deviceNameValid(s string) bool {
	if !utf8.ValidString(s) || len([]rune(s)) > 80 {
		return false
	}
	for _, c := range s {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}
func (e *Engine) compileDevicePolicies(c Config) error {
	e.devices = devicePolicies{devices: map[string]DevicePolicy{}, groups: map[string]DevicePolicyGroup{}, rules: map[string]*Engine{}}
	a := c.Advanced
	if a == nil {
		return nil
	}
	if len(a.Devices) > 256 || len(a.Groups) > 64 || len(a.Rules) > 2048 {
		return fmt.Errorf("advanced DNS limit: 256 devices, 64 groups, 2048 domain rules")
	}
	seenMACs := map[string]bool{}
	validateMAC := func(raw string) error {
		m, err := CanonicalDeviceMAC(raw)
		if err != nil || m != raw {
			return fmt.Errorf("MAC must be canonical lower-case unicast: %q", raw)
		}
		seenMACs[raw] = true
		if len(seenMACs) > 256 {
			return fmt.Errorf("advanced DNS exceeds 256 distinct devices")
		}
		return nil
	}
	target := func(t DeviceTarget) error {
		d := Decision{Matched: true, Source: "policy", Upstream: t.Upstream, GroupID: t.GroupID}
		chain, err := e.UpstreamChainForDecision(d)
		if err != nil {
			return err
		}
		for _, u := range chain {
			if u.Transport == "native" {
				return fmt.Errorf("device policy requires direct DNS, not native profiles")
			}
		}
		e.devices.targets = append(e.devices.targets, d)
		return nil
	}
	for _, d := range a.Devices {
		if err := validateMAC(d.MAC); err != nil {
			return err
		}
		if !deviceNameValid(d.Name) {
			return fmt.Errorf("invalid device name")
		}
		if _, ok := e.devices.devices[d.MAC]; ok {
			return fmt.Errorf("duplicate device MAC %s", d.MAC)
		}
		if d.Target != nil {
			if err := target(*d.Target); err != nil {
				return fmt.Errorf("device %s: %w", d.MAC, err)
			}
			e.devices.active = true
		}
		if d.Target != nil {
			copyTarget := *d.Target
			d.Target = &copyTarget
		}
		e.devices.devices[d.MAC] = d
	}
	ids := map[string]bool{}
	for _, g := range a.Groups {
		if !validID(g.ID) || ids[g.ID] || strings.TrimSpace(g.Name) == "" || !deviceNameValid(g.Name) || len(g.MACs) > 256 {
			return fmt.Errorf("invalid/duplicate device group")
		}
		ids[g.ID] = true
		if g.Target != nil {
			if err := target(*g.Target); err != nil {
				return err
			}
			e.devices.active = e.devices.active || len(g.MACs) > 0
		}
		if g.Target != nil {
			copyTarget := *g.Target
			g.Target = &copyTarget
		}
		g.MACs = append([]string{}, g.MACs...)
		for _, mac := range g.MACs {
			if err := validateMAC(mac); err != nil {
				return err
			}
			if _, ok := e.devices.groups[mac]; ok {
				return fmt.Errorf("MAC %s belongs to multiple device groups", mac)
			}
			e.devices.groups[mac] = g
		}
	}
	rules := map[string][]Rule{}
	ruleIDs := map[string]bool{}
	for _, r := range a.Rules {
		if err := validateMAC(r.MAC); err != nil {
			return err
		}
		if !validID(r.ID) || ruleIDs[r.ID] {
			return fmt.Errorf("invalid/duplicate device rule id")
		}
		ruleIDs[r.ID] = true
		if err := target(r.Target); err != nil {
			return err
		}
		rules[r.MAC] = append(rules[r.MAC], Rule{ID: r.ID, Domain: r.Domain, Match: r.Match, Priority: r.Priority, Upstream: r.Target.Upstream, GroupID: r.Target.GroupID})
	}
	for mac, rs := range rules {
		// Reuse the tested exact/suffix/subdomains matcher, with no recursive
		// Advanced config and no modifications to global rule precedence.
		compiled, err := Compile(Config{SchemaVersion: SchemaVersion, Upstreams: c.Upstreams, Groups: c.Groups, Rules: rs})
		if err != nil {
			return fmt.Errorf("device %s rules: %w", mac, err)
		}
		e.devices.rules[mac] = compiled
		e.devices.active = true
	}
	return nil
}
func (e *Engine) DevicePoliciesActive() bool { return e != nil && e.devices.active }
func (e *Engine) DeviceTargets() []Decision  { return append([]Decision{}, e.devices.targets...) }
func (e *Engine) EvaluateClientDNSQuestion(name, mac string) (ClientDecision, error) {
	common, err := e.EvaluateDNSQuestion(name)
	hostname := err == nil
	if err != nil {
		common = Decision{Domain: strings.TrimSuffix(strings.ToLower(name), "."), Source: "system", Reason: "DNS wire name has no hostname domain rule"}
	}
	if isProtected(common.Domain) || common.Domain == "my.keenetic.net" || strings.HasSuffix(common.Domain, ".my.keenetic.net") {
		return ClientDecision{Decision: Decision{Domain: common.Domain, Source: "system", Reason: "protected local namespace"}, Layer: "local_namespace"}, nil
	}
	general := func(inherited bool) ClientDecision {
		d := common
		layer := "common_domain"
		if !d.Matched {
			if e.service == nil {
				return ClientDecision{Decision: d, Layer: "system"}
			}
			d = Decision{Domain: common.Domain, Matched: true, Source: "policy", Upstream: e.service.DefaultUpstream, GroupID: e.service.DefaultGroup, Reason: "explicit default DNS"}
			layer = "default"
		}
		return ClientDecision{Decision: d, Layer: layer, DeviceMAC: mac, Inherited: inherited}
	}
	if mac == "" {
		return general(false), nil
	}
	canonical, err := CanonicalDeviceMAC(mac)
	if err != nil || canonical != mac {
		return ClientDecision{}, fmt.Errorf("invalid selected MAC")
	}
	if rules := e.devices.rules[mac]; rules != nil && hostname {
		d, err := rules.EvaluateDNSQuestion(name)
		if err != nil {
			return ClientDecision{}, err
		}
		if d.Matched {
			d.Reason = "explicit device domain rule wins before device/group/common defaults"
			return ClientDecision{Decision: d, Layer: "device_domain", DeviceMAC: mac}, nil
		}
	}
	selectTarget := func(t DeviceTarget, inherit bool, layer, group string) ClientDecision {
		if inherit && common.Matched {
			d := general(true)
			d.DeviceGroup = group
			d.Reason = "common domain rule inherited explicitly before this device/group default"
			return d
		}
		u := e.upstreams[t.Upstream]
		reason := layer + " default wins before common domains; inheritance is disabled"
		if inherit {
			reason = layer + " default: no common domain match to inherit"
		}
		return ClientDecision{Decision: Decision{Domain: common.Domain, Matched: true, Source: "policy", Upstream: t.Upstream, GroupID: t.GroupID, Transport: u.Transport, Address: u.Address, Reason: reason}, Layer: layer, DeviceMAC: mac, DeviceGroup: group}
	}
	if d, ok := e.devices.devices[mac]; ok && d.Target != nil {
		return selectTarget(*d.Target, d.InheritDomains, "device", ""), nil
	}
	if g, ok := e.devices.groups[mac]; ok && g.Target != nil {
		return selectTarget(*g.Target, g.InheritDomains, "device_group", g.ID), nil
	}
	return general(false), nil
}
