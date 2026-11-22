package lite

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Link-local identity always includes the interface scope. Related records
// preserve bridge/lower relationships for later scoped interception support.
type TopologyInterface struct {
	Name       string
	Up, Bridge bool
	Master     string
	Lower      []string
	Addresses  []string
}
type ScopedIPv6 struct {
	Address       string `json:"address"`
	Interface     string `json:"interface"`
	BridgeRelated bool   `json:"bridge_related"`
}
type GateTopology struct {
	IPv4      string       `json:"ipv4"`
	IPv6      []string     `json:"ipv6_routed"`
	LinkLocal []ScopedIPv6 `json:"ipv6_link_local"`
}

func readGateInterfaces() ([]TopologyInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	out := make([]TopologyInterface, 0, len(interfaces))
	for _, i := range interfaces {
		t := TopologyInterface{Name: i.Name, Up: i.Flags&net.FlagUp != 0}
		if !t.Up {
			out = append(out, t)
			continue
		}
		root := filepath.Join("/sys/class/net", i.Name)
		if fi, e := os.Stat(filepath.Join(root, "bridge")); e == nil {
			t.Bridge = fi.IsDir()
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		if master, e := os.Readlink(filepath.Join(root, "master")); e == nil {
			t.Master = filepath.Base(master)
		} else if !os.IsNotExist(e) {
			return nil, e
		}
		entries, e := os.ReadDir(root)
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "lower_") {
				t.Lower = append(t.Lower, strings.TrimPrefix(entry.Name(), "lower_"))
			}
		}
		addresses, e := i.Addrs()
		if e != nil {
			return nil, e
		}
		for _, a := range addresses {
			if n, ok := a.(*net.IPNet); ok {
				t.Addresses = append(t.Addresses, n.String())
			}
		}
		out = append(out, t)
	}
	return out, nil
}
func DiscoverGateTopology() (GateTopology, error) {
	interfaces, err := readGateInterfaces()
	if err != nil {
		return GateTopology{}, err
	}
	return AnalyzeGateTopology(interfaces)
}

// AnalyzeGateTopology is pure. Scoped fe80 addresses are recorded separately,
// never entered into an unscoped ipset or treated as routed-prefix collisions.
func AnalyzeGateTopology(interfaces []TopologyInterface) (GateTopology, error) {
	var result GateTopology
	byName := map[string]TopologyInterface{}
	for _, i := range interfaces {
		if _, dup := byName[i.Name]; dup {
			return result, errors.New("duplicate topology interface")
		}
		byName[i.Name] = i
	}
	bridge, ok := byName[gateInterface]
	if !ok || !bridge.Up || !bridge.Bridge {
		return result, errors.New("managed br0 kernel LAN bridge unavailable")
	}
	related := map[string]bool{gateInterface: true}
	for changed := true; changed; {
		changed = false
		for _, i := range interfaces {
			if i.Master != "" && related[i.Master] && !related[i.Name] {
				related[i.Name] = true
				changed = true
			}
			if related[i.Name] {
				for _, lower := range i.Lower {
					if !related[lower] {
						related[lower] = true
						changed = true
					}
				}
			}
		}
	}
	for _, raw := range bridge.Addresses {
		ip, n, err := net.ParseCIDR(raw)
		if err != nil {
			return result, fmt.Errorf("invalid br0 address %q", raw)
		}
		if ip.To4() != nil {
			if result.IPv4 != "" {
				return result, errors.New("multiple br0 IPv4 prefixes require explicit audit")
			}
			ones, _ := n.Mask.Size()
			if !ip.IsPrivate() || ones < 8 {
				return result, errors.New("managed IPv4 bridge must use a private prefix")
			}
			result.IPv4 = raw
		} else if ip.IsLinkLocalUnicast() {
			// Scope is retained below for every interface, including this bridge.
		} else if ip.IsGlobalUnicast() {
			ones, _ := n.Mask.Size()
			if ones < 32 {
				return result, errors.New("managed IPv6 prefix is too broad")
			}
			result.IPv6 = append(result.IPv6, raw)
		}
	}
	if result.IPv4 == "" {
		return result, errors.New("br0 IPv4 LAN prefix required")
	}
	if len(result.IPv6) > 4 {
		return result, errors.New("too many br0 IPv6 routed prefixes")
	}
	managed := append([]string{result.IPv4}, result.IPv6...)
	for _, i := range interfaces {
		if !i.Up {
			continue
		}
		for _, raw := range i.Addresses {
			ip, n, err := net.ParseCIDR(raw)
			if err != nil {
				return result, fmt.Errorf("invalid %s address %q", i.Name, raw)
			}
			if ip.To4() == nil && ip.IsLinkLocalUnicast() {
				result.LinkLocal = append(result.LinkLocal, ScopedIPv6{Address: raw, Interface: i.Name, BridgeRelated: related[i.Name]})
				continue
			}
			if i.Name == gateInterface {
				continue
			}
			for _, own := range managed {
				ownIP, ownNet, _ := net.ParseCIDR(own)
				if ownNet.Contains(ip) || n.Contains(ownIP) {
					return result, fmt.Errorf("managed LAN routed prefix overlaps %s; conntrack scope is ambiguous", i.Name)
				}
			}
		}
	}
	sort.Strings(result.IPv6)
	sort.Slice(result.LinkLocal, func(i, j int) bool {
		if result.LinkLocal[i].Interface != result.LinkLocal[j].Interface {
			return result.LinkLocal[i].Interface < result.LinkLocal[j].Interface
		}
		return result.LinkLocal[i].Address < result.LinkLocal[j].Address
	})
	return result, nil
}
func GateCandidates() (string, []string, error) {
	t, err := DiscoverGateTopology()
	return t.IPv4, t.IPv6, err
}
func (t GateTopology) legacyCandidates() (string, []string, error) {
	v6 := append([]string{}, t.IPv6...)
	for _, a := range t.LinkLocal {
		if a.Interface == gateInterface {
			v6 = append(v6, a.Address)
		}
	}
	return t.IPv4, v6, nil
}
