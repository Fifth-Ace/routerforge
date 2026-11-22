package lite

import (
	"net"
	"strconv"
	"strings"
)

// P19.2: classify the potential LAN DNS path, not an arbitrary mangle jump.
// We only discard rules proven incapable of matching UDP/TCP :53 on the
// managed ingress. Unknown matches remain possible (fail-closed).
func mangleRuleValue(r []string, names ...string) (value string, found, inverted bool) {
	for i, arg := range r {
		for _, name := range names {
			if arg != name {
				continue
			}
			j := i + 1
			inverted = i > 0 && r[i-1] == "!"
			if j < len(r) && r[j] == "!" {
				inverted = true
				j++
			}
			if j >= len(r) {
				return "", true, inverted
			}
			return r[j], true, inverted
		}
	}
	return "", false, false
}

func mangleInterfaceMatches(pattern, ingress string) bool {
	if pattern == "" {
		return true // malformed / unrecognised: cannot prove exclusion
	}
	if strings.HasSuffix(pattern, "+") {
		return strings.HasPrefix(ingress, strings.TrimSuffix(pattern, "+"))
	}
	return pattern == ingress
}

func manglePortSetMayMatch(raw string, port int) (match, known bool) {
	if raw == "" {
		return true, false
	}
	for _, token := range strings.Split(raw, ",") {
		parts := strings.Split(token, ":")
		if len(parts) < 1 || len(parts) > 2 {
			return true, false
		}
		first, err := strconv.Atoi(parts[0])
		if err != nil || first < 0 || first > 65535 {
			return true, false
		}
		last := first
		if len(parts) == 2 {
			last, err = strconv.Atoi(parts[1])
			if err != nil || last < first || last > 65535 {
				return true, false
			}
		}
		if first <= port && port <= last {
			return true, true
		}
	}
	return false, true
}

func mangleSourceMayMatch(raw string, inverted bool, prefixes []string) bool {
	ip := net.ParseIP(raw)
	var n *net.IPNet
	if ip != nil {
		bits := 128
		if ip.To4() != nil {
			bits = 32
		}
		if bits == 32 {
			ip = ip.To4()
		}
		n = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
	} else {
		_, n, _ = net.ParseCIDR(raw)
	}
	if n == nil || len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		_, owned, err := net.ParseCIDR(prefix)
		if err != nil {
			return true
		}
		if inverted {
			// Only exclude when every possible source is inside the excluded net.
			if !n.Contains(owned.IP) || !n.Contains(mangleLastIPInNet(owned)) {
				return true
			}
		} else if n.Contains(owned.IP) || owned.Contains(n.IP) {
			return true
		}
	}
	return false
}

func mangleLastIPInNet(n *net.IPNet) net.IP {
	out := append(net.IP(nil), n.IP...)
	for i := range out {
		out[i] |= ^n.Mask[i]
	}
	return out
}

func mangleRuleCanMatchDNS(r []string, ingress, protocol string, prefixes []string) bool {
	if iface, ok, not := mangleRuleValue(r, "-i", "--in-interface"); ok {
		eq := mangleInterfaceMatches(iface, ingress)
		if not {
			eq = !eq
		}
		if !eq {
			return false
		}
	}
	if proto, ok, not := mangleRuleValue(r, "-p", "--protocol"); ok {
		proto = strings.ToLower(proto)
		if proto == "6" {
			proto = "tcp"
		} else if proto == "17" {
			proto = "udp"
		}
		eq := proto == protocol || proto == "all" || proto == "0" || proto == ""
		if not {
			eq = !eq
		}
		if !eq {
			return false
		}
	}
	for _, spec := range []struct{ names []string }{
		{[]string{"--dport", "--destination-port"}},
		{[]string{"--dports", "--destination-ports"}},
	} {
		if port, ok, not := mangleRuleValue(r, spec.names...); ok {
			matches, known := manglePortSetMayMatch(port, 53)
			if known && ((!not && !matches) || (not && matches)) {
				return false
			}
		}
	}
	// --sports/--sport constrain the CLIENT SOURCE port. It is not fixed by
	// DNS protocol, so never misread them as an exclusion of destination :53.
	if src, ok, not := mangleRuleValue(r, "-s", "--source"); ok && !mangleSourceMayMatch(src, not, prefixes) {
		return false
	}
	return true
}

// inspectMangleDNS is read-only and family-scoped. Names of third-party
// modules and WAN interfaces are intentionally not hard-coded.
func inspectMangleDNS(dump string, p GatePlan, f GateFamily) error {
	chains := map[string][][]string{}
	for _, line := range strings.Split(dump, "\n") {
		r := words(line)
		if argValue(r, "--match-set") == f.Set && f.Set != "" {
			return conflict("foreign mangle reference to owned lease set: %s", line)
		}
		if len(r) == 2 && r[0] == "-N" {
			if _, exists := chains[r[1]]; !exists {
				chains[r[1]] = nil
			}
		}
		if len(r) >= 3 && r[0] == "-A" {
			chains[r[1]] = append(chains[r[1]], r)
		}
	}
	var visit func(string, string, map[string]bool) error
	visit = func(chain, protocol string, visiting map[string]bool) error {
		if visiting[chain] {
			return conflict("possible cyclic mangle DNS path at %s", chain)
		}
		visiting[chain] = true
		defer delete(visiting, chain)
		for _, r := range chains[chain] {
			if !mangleRuleCanMatchDNS(r, p.Interface, protocol, f.Prefixes) {
				continue
			}
			target := argValue(r, "-j")
			if target == "" {
				target = argValue(r, "-g")
			}
			if strings.HasPrefix(target, "_NDM_") {
				continue // existing Keenetic-owned mangle handling
			}
			switch target {
			case "", "RETURN", "ACCEPT", "LOG", "NFLOG", "TRACE":
				continue
			case "NFQUEUE", "QUEUE", "TPROXY", "REDIRECT", "DNAT", "DROP", "REJECT", "NOTRACK", "MARK", "CONNMARK", "CONNNDMMARK", "NDMMARK":
				return conflict("competing mangle DNS interception (%s/%s): %s", p.Interface, protocol, ruleString(r))
			default:
				if _, ok := chains[target]; ok {
					if err := visit(target, protocol, visiting); err != nil {
						return err
					}
					continue
				}
				return conflict("unclassified mangle DNS target %q (%s/%s): %s", target, p.Interface, protocol, ruleString(r))
			}
		}
		return nil
	}
	for _, protocol := range []string{"udp", "tcp"} {
		if err := visit("PREROUTING", protocol, map[string]bool{}); err != nil {
			return err
		}
	}
	return nil
}
