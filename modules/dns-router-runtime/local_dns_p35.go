package dnsrouter

// P35 Local DNS: opt-in exact records under the dedicated rf.home.arpa
// namespace.  Stock Keenetic names and reverse lookups remain untouched.
import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

const P35Zone = "rf.home.arpa"
const P35MaxRecords = 128

type LocalDNSConfig struct {
	Enabled bool             `json:"enabled"`
	Records []LocalDNSRecord `json:"records,omitempty"`
}

type LocalDNSRecord struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
	TTL   uint32 `json:"ttl"`
}

func localDNSNameP35(name string) (string, error) {
	if name != strings.ToLower(name) || strings.HasSuffix(name, ".") {
		return "", fmt.Errorf("P35 names must use lowercase DNS format without trailing dot")
	}
	clean, err := normalizedDomain(name)
	if err != nil || clean != name {
		return "", fmt.Errorf("P35 invalid DNS name %q", name)
	}
	return clean, nil
}

func localDNSOwnerP35(name string) bool {
	return name != P35Zone && strings.HasSuffix(name, "."+P35Zone)
}

func localDNSPrivateP35(ip netip.Addr) bool {
	return ip.IsValid() && ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsUnspecified() && !ip.Is4In6()
}

// ReverseDNSNameP35 produces a canonical reverse owner for a private IP.
func ReverseDNSNameP35(ip netip.Addr) (string, error) {
	if !localDNSPrivateP35(ip) {
		return "", fmt.Errorf("P35 PTR requires a private unicast address")
	}
	if ip.Is4() {
		a := ip.As4()
		return strconv.Itoa(int(a[3])) + "." + strconv.Itoa(int(a[2])) + "." + strconv.Itoa(int(a[1])) + "." + strconv.Itoa(int(a[0])) + ".in-addr.arpa", nil
	}
	a := ip.As16()
	var b strings.Builder
	const hex = "0123456789abcdef"
	for i := 15; i >= 0; i-- {
		b.WriteByte(hex[a[i]&15])
		b.WriteByte('.')
		b.WriteByte(hex[a[i]>>4])
		b.WriteByte('.')
	}
	b.WriteString("ip6.arpa")
	return b.String(), nil
}

func validateLocalDNSP35(c *LocalDNSConfig, listen ...string) error {
	var listener netip.Addr
	if len(listen) > 0 {
		host, _, err := net.SplitHostPort(listen[0])
		if err == nil {
			listener, _ = netip.ParseAddr(host)
		}
	}
	if c == nil {
		return nil
	}
	if len(c.Records) > P35MaxRecords {
		return fmt.Errorf("P35 limit: max %d local records", P35MaxRecords)
	}
	if !c.Enabled && len(c.Records) > 0 {
		return fmt.Errorf("P35 disabled configuration cannot contain records")
	}
	if !c.Enabled {
		return nil
	}
	seen := make(map[string]bool, len(c.Records))
	owners := make(map[string]map[string]bool)
	// Only explicit forward records can own a private reverse PTR.
	addrs := make(map[string]string)
	cnames := make(map[string]string)
	for _, r := range c.Records {
		if r.TTL < 30 || r.TTL > 86400 {
			return fmt.Errorf("P35 TTL must be 30..86400 seconds")
		}
		if r.Type != "A" && r.Type != "AAAA" && r.Type != "CNAME" && r.Type != "PTR" {
			return fmt.Errorf("P35 unsupported record type %q", r.Type)
		}
		owner, err := localDNSNameP35(r.Name)
		if err != nil {
			return err
		}
		if r.Type != "PTR" && !localDNSOwnerP35(owner) {
			return fmt.Errorf("P35 may only own names below *.%s", P35Zone)
		}
		key := owner + "|" + r.Type
		if seen[key] {
			return fmt.Errorf("P35 duplicate name/type: %s", key)
		}
		seen[key] = true
		if owners[owner] == nil {
			owners[owner] = map[string]bool{}
		}
		owners[owner][r.Type] = true
		switch r.Type {
		case "A", "AAAA":
			ip, e := netip.ParseAddr(r.Value)
			if e != nil || !localDNSPrivateP35(ip) || (r.Type == "A" && !ip.Is4()) || (r.Type == "AAAA" && !ip.Is6()) || ip.String() != r.Value {
				return fmt.Errorf("P35 %s requires canonical private unicast IP", r.Type)
			}
			addrs[ip.String()] = owner
		case "CNAME":
			target, e := localDNSNameP35(r.Value)
			if e != nil || !localDNSOwnerP35(target) {
				return fmt.Errorf("P35 CNAME target must be inside *.%s", P35Zone)
			}
			if target == owner {
				return fmt.Errorf("P35 self-referencing CNAME")
			}
			cnames[owner] = target
		case "PTR":
			target, e := localDNSNameP35(r.Value)
			if e != nil || !localDNSOwnerP35(target) {
				return fmt.Errorf("P35 PTR target must be inside *.%s", P35Zone)
			}
		}
	}
	for owner, types := range owners {
		if types["CNAME"] && len(types) != 1 {
			return fmt.Errorf("P35 CNAME cannot coexist with other types: %s", owner)
		}
	}
	for _, r := range c.Records {
		if r.Type != "PTR" {
			continue
		}
		matched := false
		for ip, name := range addrs {
			if name != r.Value {
				continue
			}
			addr, e := netip.ParseAddr(ip)
			if e != nil {
				continue
			}
			reverse, e := ReverseDNSNameP35(addr)
			if e == nil && reverse == r.Name {
				if listener.IsValid() && addr == listener {
					return fmt.Errorf("P35 PTR cannot override the DNS listener reverse name")
				}
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("P35 PTR must match an explicitly defined local A/AAAA record: %s", r.Name)
		}
	}
	for name := range cnames {
		visited := map[string]bool{}
		current := name
		for i := 0; i <= len(cnames); i++ {
			next, exists := cnames[current]
			if !exists {
				break
			}
			if visited[current] {
				return fmt.Errorf("P35 CNAME cycle: %s", name)
			}
			visited[current] = true
			current = next
		}
	}
	return nil
}
