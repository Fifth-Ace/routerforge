package lite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

const deviceFreshness = 20 * time.Second

// An observation is evidence, not authentication. DHCP alone is never trusted.
type DeviceObservation struct {
	IP, MAC, Interface, State, Name, Source string
	ObservedAt, ValidUntil                  time.Time
}
type DeviceSnapshot struct {
	Neighbors, Leases []DeviceObservation
	Interfaces        []TopologyInterface
	Errors            []string
}
type DeviceIdentity struct {
	IP         string    `json:"ip"`
	MAC        string    `json:"mac,omitempty"`
	Name       string    `json:"name,omitempty"`
	Interface  string    `json:"interface,omitempty"`
	Trusted    bool      `json:"trusted"`
	State      string    `json:"state"`
	Source     string    `json:"source,omitempty"`
	Reason     string    `json:"reason"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
	ValidUntil time.Time `json:"valid_until,omitempty"`
}
type DeviceDirectory struct {
	mu          sync.RWMutex
	refresh     sync.Mutex
	Read        func(context.Context) (DeviceSnapshot, error)
	Now         func() time.Time
	entries     map[string]DeviceIdentity
	errors      []string
	lastAttempt time.Time
}

func NewDeviceDirectory() *DeviceDirectory {
	return &DeviceDirectory{Read: readDeviceSnapshot, Now: time.Now, entries: map[string]DeviceIdentity{}}
}
func unknownDevice(ip, why string) DeviceIdentity {
	return DeviceIdentity{IP: ip, State: "unknown", Reason: why}
}
func (d *DeviceDirectory) Resolve(ip string) DeviceIdentity {
	if d == nil {
		return unknownDevice(ip, "IP/MAC discovery unavailable; common policy")
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return unknownDevice(ip, "invalid client IP; common policy")
	}
	addr = addr.Unmap()
	if addr.IsLoopback() {
		return unknownDevice(ip, "local/proxied request; end-device MAC unavailable")
	}
	if addr.IsLinkLocalUnicast() && addr.Zone() == "" {
		return unknownDevice(ip, "IPv6 link-local interface scope unavailable")
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	row, ok := d.entries[addr.String()]
	if !ok {
		return unknownDevice(ip, "no fresh direct LAN neighbor; NAT/VPN downstream clients cannot be inferred")
	}
	now := d.Now()
	if !row.Trusted || !now.Before(row.ValidUntil) || row.ObservedAt.After(now) {
		row.MAC = ""
		row.Name = ""
		row.Trusted = false
		if !now.Before(row.ValidUntil) {
			row.State = "expired"
			row.Reason = "IP/MAC observation expired; common policy"
		}
	}
	return row
}
func (d *DeviceDirectory) Snapshot() ([]DeviceIdentity, []string) {
	if d == nil {
		return []DeviceIdentity{}, []string{"discovery not initialized"}
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	rows := make([]DeviceIdentity, 0, len(d.entries))
	now := d.Now()
	for _, r := range d.entries {
		if !now.Before(r.ValidUntil) {
			r.Trusted = false
			r.State = "expired"
			r.Reason = "observation expired"
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].IP < rows[j].IP })
	if len(rows) > 512 {
		rows = rows[:512]
	}
	return rows, append([]string{}, d.errors...)
}
func (d *DeviceDirectory) Refresh(ctx context.Context) {
	if d == nil || !d.refresh.TryLock() {
		return
	}
	defer d.refresh.Unlock()
	now := d.Now()
	d.mu.RLock()
	recent := now.Sub(d.lastAttempt) < 2*time.Second && !d.lastAttempt.IsZero()
	d.mu.RUnlock()
	if recent {
		return
	}
	bounded, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	snapshot, err := d.Read(bounded)
	if err != nil {
		snapshot = DeviceSnapshot{Errors: []string{err.Error()}}
	}
	entries := buildDeviceIdentities(snapshot, now)
	d.mu.Lock()
	d.entries = entries
	d.errors = append([]string{}, snapshot.Errors...)
	d.lastAttempt = now
	d.mu.Unlock()
}

// Replace is an injected snapshot entry point for tests; production uses Refresh.
func (d *DeviceDirectory) replace(s DeviceSnapshot) {
	now := d.Now()
	entries := buildDeviceIdentities(s, now)
	d.mu.Lock()
	d.entries = entries
	d.errors = append([]string{}, s.Errors...)
	d.mu.Unlock()
}
func buildDeviceIdentities(s DeviceSnapshot, now time.Time) map[string]DeviceIdentity {
	interfaces := map[string]TopologyInterface{}
	bridges := map[string][]*net.IPNet{}
	own := map[string]bool{}
	for _, i := range s.Interfaces {
		interfaces[i.Name] = i
		if !i.Up || !i.Bridge {
			continue
		}
		for _, raw := range i.Addresses {
			ip, n, err := net.ParseCIDR(raw)
			if err != nil {
				continue
			}
			if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil && !ip.IsPrivate() {
				continue
			}
			bridges[i.Name] = append(bridges[i.Name], n)
			own[ip.String()] = true
		}
	}
	// Canonicalize actual bridge members, never infer membership from a VLAN's
	// lower device: that lower device may also carry WAN or unrelated VLANs.
	bridgeFor := func(iface string) string {
		seen := map[string]bool{}
		for iface != "" && !seen[iface] {
			seen[iface] = true
			if _, ok := bridges[iface]; ok {
				return iface
			}
			i, ok := interfaces[iface]
			if !ok || !i.Up {
				return ""
			}
			iface = i.Master
		}
		return ""
	}
	out := map[string]DeviceIdentity{}
	for _, o := range s.Neighbors {
		bridge := bridgeFor(o.Interface)
		if bridge == "" {
			continue
		}
		a, err := netip.ParseAddr(o.IP)
		if err != nil {
			continue
		}
		a = a.Unmap()
		if a.IsUnspecified() || a.IsMulticast() || a.IsLoopback() || own[a.WithZone("").String()] {
			continue
		}
		if a.Zone() != "" && a.Zone() != o.Interface && a.Zone() != bridge {
			continue
		}
		contained := false
		for _, n := range bridges[bridge] {
			contained = contained || n.Contains(net.IP(a.AsSlice()))
		}
		if !contained {
			continue
		}
		if a.IsLinkLocalUnicast() {
			a = a.WithZone(bridge)
		} else {
			a = a.WithZone("")
		}
		key := a.String()
		mac, err := dr.CanonicalDeviceMAC(o.MAC)
		if err != nil {
			continue
		}
		expires := o.ValidUntil
		if max := o.ObservedAt.Add(deviceFreshness); expires.IsZero() || max.Before(expires) {
			expires = max
		}
		trusted := (o.State == "REACHABLE" || o.State == "PERMANENT") && !o.ObservedAt.After(now) && now.Before(expires) && !o.ObservedAt.IsZero()
		row := DeviceIdentity{IP: key, MAC: mac, Interface: bridge, Trusted: trusted, State: o.State, Source: "kernel-neighbor", Reason: "fresh direct LAN neighbor; MAC is not authentication", ObservedAt: o.ObservedAt, ValidUntil: expires}
		if !trusted {
			row.Reason = "neighbor is stale, incomplete or expired; common policy"
		}
		for _, lease := range s.Leases {
			ip, e := netip.ParseAddr(lease.IP)
			if e != nil || ip.Unmap() != a.WithZone("") {
				continue
			}
			if lease.ObservedAt.After(now) || !now.Before(lease.ValidUntil) || now.Sub(lease.ObservedAt) > deviceFreshness {
				continue
			}
			leaseMAC, e := dr.CanonicalDeviceMAC(lease.MAC)
			if e != nil {
				continue
			}
			if leaseMAC != mac {
				row.Trusted = false
				row.State = "conflict"
				row.Reason = "DHCP and neighbor MAC disagree; common policy"
			} else {
				row.Source = "kernel-neighbor+dhcp"
				row.Name = lease.Name
				if lease.ValidUntil.Before(row.ValidUntil) {
					row.ValidUntil = lease.ValidUntil
				}
			}
		}
		if previous, ok := out[key]; ok {
			if previous.MAC != row.MAC || previous.Interface != row.Interface || previous.State == "conflict" {
				row.Trusted = false
				row.State = "conflict"
				row.Reason = "ambiguous IP/MAC or interface observations; common policy"
			} else if !previous.Trusted {
				row.Trusted = false
				row.Reason = previous.Reason
			}
			if previous.ValidUntil.Before(row.ValidUntil) {
				row.ValidUntil = previous.ValidUntil
			}
		}
		out[key] = row
	}
	return out
}
func parseNeighborTable(raw string, now time.Time) ([]DeviceObservation, error) {
	if len(raw) > 512<<10 {
		return nil, errors.New("neighbor table exceeds limit")
	}
	out := []DeviceObservation{}
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		if _, e := netip.ParseAddr(f[0]); e != nil {
			return nil, fmt.Errorf("unrecognized neighbor table format")
		}
		iface, mac, state := "", "", "UNKNOWN"
		for i := 1; i < len(f); i++ {
			switch f[i] {
			case "dev":
				if i+1 < len(f) {
					iface = f[i+1]
				}
			case "lladdr":
				if i+1 < len(f) {
					mac = f[i+1]
				}
			case "REACHABLE", "PERMANENT", "STALE", "DELAY", "PROBE", "FAILED", "INCOMPLETE", "NOARP":
				state = f[i]
			}
		}
		if iface == "" {
			return nil, errors.New("neighbor table omitted interface scope")
		}
		if mac == "" {
			continue
		}
		out = append(out, DeviceObservation{IP: f[0], MAC: mac, Interface: iface, State: state, Source: "kernel-neighbor", ObservedAt: now, ValidUntil: now.Add(deviceFreshness)})
		if len(out) > 4096 {
			return nil, errors.New("too many neighbors")
		}
	}
	return out, nil
}

// Keenetic's documented read-only `show ip dhcp bindings` output. Leases only
// enrich names and veto conflicts, never establish identity on their own.
func parseDHCPBindings(raw string, now time.Time) ([]DeviceObservation, error) {
	if len(raw) > 512<<10 {
		return nil, errors.New("DHCP table exceeds limit")
	}
	rows := []DeviceObservation{}
	current := map[string]string{}
	flush := func() {
		seconds, e := strconv.Atoi(current["expires"])
		ip, e2 := netip.ParseAddr(current["ip"])
		mac, e3 := dr.CanonicalDeviceMAC(current["mac"])
		if e == nil && e2 == nil && ip.Is4() && e3 == nil && seconds > 0 {
			if seconds > 20 {
				seconds = 20
			}
			name := current["hostname"]
			if len(name) > 160 {
				name = ""
			}
			rows = append(rows, DeviceObservation{IP: ip.String(), MAC: mac, Name: name, Source: "dhcp", State: "lease", ObservedAt: now, ValidUntil: now.Add(time.Duration(seconds) * time.Second)})
		}
	}
	for _, line := range strings.Split(raw, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		if k == "lease" {
			flush()
			current = map[string]string{}
		} else if k == "ip" || k == "mac" || k == "hostname" || k == "expires" {
			current[k] = strings.TrimSpace(v)
		}
	}
	flush()
	if len(rows) > 4096 {
		return nil, errors.New("too many DHCP bindings")
	}
	return rows, nil
}
func readDeviceSnapshot(ctx context.Context) (DeviceSnapshot, error) {
	s := DeviceSnapshot{}
	interfaces, err := readGateInterfaces()
	if err != nil {
		return s, err
	}
	s.Interfaces = interfaces
	now := time.Now()
	for _, family := range []string{"-4", "-6"} {
		raw, e := systemFirewallIO(ctx, "ip", family, "neigh", "show")
		if e != nil {
			s.Errors = append(s.Errors, family+" neighbors unavailable: "+e.Error())
			continue
		}
		rows, e := parseNeighborTable(raw, now)
		if e != nil {
			s.Errors = append(s.Errors, e.Error())
		} else {
			s.Neighbors = append(s.Neighbors, rows...)
		}
	}
	raw, e := systemFirewallIO(ctx, "ndmc", "-c", "show ip dhcp bindings")
	if e != nil {
		s.Errors = append(s.Errors, "DHCP names/conflict cross-check unavailable")
	} else {
		rows, e := parseDHCPBindings(raw, now)
		if e != nil {
			s.Errors = append(s.Errors, e.Error())
		} else {
			s.Leases = rows
		}
	}
	return s, nil
}
