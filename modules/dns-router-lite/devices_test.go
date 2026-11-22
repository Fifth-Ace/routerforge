package lite

import (
	"context"
	"errors"
	"testing"
	"time"
)

const testMACA = "02:11:22:33:44:55"
const testMACB = "02:11:22:33:44:66"

func deviceTopology() []TopologyInterface {
	return []TopologyInterface{
		{Name: "br0", Up: true, Bridge: true, Addresses: []string{"192.168.10.1/24", "fd01::1/64", "fe80::1/64"}},
		{Name: "ra0", Master: "br0", Up: true}, {Name: "eth2.1", Master: "br0", Lower: []string{"eth2"}, Up: true}, {Name: "eth2", Up: true},
		{Name: "wan-other", Up: true, Addresses: []string{"192.168.10.9/24"}},
	}
}
func observed(ip, mac, iface, state string, now time.Time) DeviceObservation {
	return DeviceObservation{IP: ip, MAC: mac, Interface: iface, State: state, ObservedAt: now, ValidUntil: now.Add(deviceFreshness)}
}
func TestP21NeighborScopesFreshnessIPv4IPv6AndDHCP(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	d := NewDeviceDirectory()
	d.Now = func() time.Time { return now }
	s := DeviceSnapshot{Interfaces: deviceTopology(), Neighbors: []DeviceObservation{
		observed("192.168.10.22", testMACA, "ra0", "REACHABLE", now), observed("fd01::22", testMACA, "br0", "REACHABLE", now), observed("fe80::22", testMACA, "eth2.1", "REACHABLE", now),
		observed("192.168.10.44", testMACB, "eth2", "REACHABLE", now), observed("192.168.10.45", testMACB, "wan-other", "REACHABLE", now), observed("192.168.10.1", testMACA, "br0", "REACHABLE", now), observed("192.168.10.99", testMACB, "br0", "STALE", now),
	}, Leases: []DeviceObservation{{IP: "192.168.10.22", MAC: testMACA, Name: "Laptop", ObservedAt: now, ValidUntil: now.Add(15 * time.Second)}}}
	d.replace(s)
	for _, ip := range []string{"192.168.10.22", "fd01::22", "fe80::22%br0"} {
		r := d.Resolve(ip)
		if !r.Trusted || r.MAC != testMACA {
			t.Fatal(ip, r)
		}
	}
	if r := d.Resolve("192.168.10.22"); r.Name != "Laptop" || r.Source != "kernel-neighbor+dhcp" {
		t.Fatal(r)
	}
	for _, ip := range []string{"192.168.10.44", "192.168.10.45", "192.168.10.1", "192.168.10.99", "192.168.10.77", "fe80::22", "fe80::22%eth2.1", "127.0.0.1"} {
		r := d.Resolve(ip)
		if r.Trusted || r.MAC != "" {
			t.Fatal("unsafe identity", ip, r)
		}
	}
	now = now.Add(21 * time.Second)
	if r := d.Resolve("fd01::22"); r.Trusted || r.MAC != "" || r.State != "expired" {
		t.Fatal(r)
	}
	// DHCP moves .22 -> .23. Old observation is vetoed; new IP uses new table.
	s.Neighbors = []DeviceObservation{observed("192.168.10.22", testMACA, "br0", "REACHABLE", now), observed("192.168.10.23", testMACA, "br0", "REACHABLE", now)}
	s.Leases = []DeviceObservation{{IP: "192.168.10.22", MAC: testMACB, ObservedAt: now, ValidUntil: now.Add(20 * time.Second)}, {IP: "192.168.10.23", MAC: testMACA, ObservedAt: now, ValidUntil: now.Add(20 * time.Second)}}
	d.replace(s)
	if r := d.Resolve("192.168.10.22"); r.Trusted || r.MAC != "" || r.State != "conflict" {
		t.Fatal(r)
	}
	if r := d.Resolve("192.168.10.23"); !r.Trusted || r.MAC != testMACA {
		t.Fatal(r)
	}
	s.Neighbors = nil
	d.replace(s)
	if r := d.Resolve("192.168.10.23"); r.Trusted || r.MAC != "" {
		t.Fatal("DHCP alone trusted", r)
	}
}
func TestP21AmbiguousNeighborsAndReadFailure(t *testing.T) {
	now := time.Now()
	d := NewDeviceDirectory()
	d.Now = func() time.Time { return now }
	for _, reverse := range []bool{false, true} {
		neighbors := []DeviceObservation{observed("192.168.10.22", testMACA, "ra0", "REACHABLE", now), observed("192.168.10.22", testMACB, "br0", "REACHABLE", now)}
		if reverse {
			neighbors[0], neighbors[1] = neighbors[1], neighbors[0]
		}
		d.replace(DeviceSnapshot{Interfaces: deviceTopology(), Neighbors: neighbors})
		if r := d.Resolve("192.168.10.22"); r.Trusted || r.MAC != "" {
			t.Fatal(r)
		}
	}
	// Independent bridge scopes stay distinct for link-local but conflict for identical routed IP.
	topology := append(deviceTopology(), TopologyInterface{Name: "br1", Up: true, Bridge: true, Addresses: []string{"192.168.10.2/24", "fe80::2/64"}})
	neighbors := []DeviceObservation{observed("192.168.10.22", testMACA, "br0", "REACHABLE", now), observed("192.168.10.22", testMACA, "br1", "REACHABLE", now), observed("fe80::22", testMACA, "br0", "REACHABLE", now), observed("fe80::22", testMACB, "br1", "REACHABLE", now)}
	d.replace(DeviceSnapshot{Interfaces: topology, Neighbors: neighbors})
	if d.Resolve("192.168.10.22").Trusted || !d.Resolve("fe80::22%br0").Trusted || d.Resolve("fe80::22%br1").MAC != testMACB {
		t.Fatal("scope conflict")
	}
	d.Read = func(context.Context) (DeviceSnapshot, error) { return DeviceSnapshot{}, errors.New("read failed") }
	d.Refresh(context.Background())
	if d.Resolve("fe80::22%br0").Trusted {
		t.Fatal("old snapshot retained after failure")
	}
}
func TestP21NeighborAndDocumentedDHCPParsers(t *testing.T) {
	now := time.Now()
	rows, err := parseNeighborTable("192.168.10.22 dev br0 lladdr 02:11:22:33:44:55 REACHABLE\nfd01::22 dev ra0 lladdr 02:11:22:33:44:55 STALE\n192.168.10.99 dev br0 FAILED", now)
	if err != nil || len(rows) != 2 || rows[1].State != "STALE" {
		t.Fatal(rows, err)
	}
	if _, err = parseNeighborTable("192.168.10.22 lladdr 02:11:22:33:44:55 REACHABLE", now); err == nil {
		t.Fatal("unscoped table accepted")
	}
	leases, err := parseDHCPBindings("lease:\n ip: 192.168.10.22\n mac: 02:11:22:33:44:55\n expires: 289\n hostname: lenovo\nlease:\n ip: 192.168.10.99\n mac: 02:11:22:33:44:66\n expires: 0", now)
	if err != nil || len(leases) != 1 || leases[0].Name != "lenovo" || leases[0].ValidUntil.Sub(now) != 20*time.Second {
		t.Fatal(leases, err)
	}
}
