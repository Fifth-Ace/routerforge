package lite

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
)

type modelCT struct {
	mu      sync.Mutex
	flows   []CTFlow
	deleted []CTFlow
	err     error
}

func (m *modelCT) Dump(context.Context) ([]CTFlow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]CTFlow{}, m.flows...), m.err
}
func (m *modelCT) Delete(_ context.Context, f CTFlow) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	for i, item := range m.flows {
		if string(item.ID) == string(f.ID) {
			m.flows = append(m.flows[:i], m.flows[i+1:]...)
			m.deleted = append(m.deleted, f)
			return nil
		}
	}
	return nil
}
func testAttr(kind uint16, p []byte) []byte {
	b := make([]byte, align4(len(p)+4))
	binary.NativeEndian.PutUint16(b, uint16(len(p)+4))
	binary.NativeEndian.PutUint16(b[2:], kind)
	copy(b[4:], p)
	return b
}
func testTuple(src, dst string, proto byte, sport, dport uint16) []byte {
	ip := net.ParseIP(src)
	other := net.ParseIP(dst)
	srcKind, dstKind := uint16(1), uint16(2)
	if ip.To4() != nil {
		ip = ip.To4()
		other = other.To4()
	} else {
		srcKind, dstKind = 3, 4
	}
	ips := append(testAttr(srcKind, ip), testAttr(dstKind, other)...)
	p := testAttr(1, []byte{proto})
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, sport)
	p = append(p, testAttr(2, b)...)
	binary.BigEndian.PutUint16(b, dport)
	p = append(p, testAttr(3, b)...)
	return append(testAttr(0x8001, ips), testAttr(0x8002, p)...)
}
func testFlow(t *testing.T, source, dest string, proto byte, port uint16, id uint32) CTFlow {
	t.Helper()
	family := byte(2)
	router := "192.168.10.1"
	if net.ParseIP(source).To4() == nil {
		family = 10
		router = "fd12:3456::1"
	}
	a := testAttr(0x8001, testTuple(source, dest, proto, 34567, 53))
	a = append(a, testAttr(0x8002, testTuple(router, source, proto, port, 34567))...)
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, id)
	a = append(a, testAttr(12, b)...)
	f, err := parseCTFlow(append([]byte{family, 0, 0, 0}, a...))
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func TestP19ConntrackExactCleanupAndUnrelatedTraffic(t *testing.T) {
	g, _, _ := gateFixture(t)
	p, e := g.makePlan("192.168.10.1:55358")
	if e != nil {
		t.Fatal(e)
	}
	own4 := testFlow(t, "192.168.10.5", "9.9.9.9", 17, 55358, 1)
	bypass4 := testFlow(t, "192.168.10.6", "192.168.10.1", 6, 53, 2)
	own6 := testFlow(t, "fd12:3456::5", "2606:4700:4700::1111", 6, 55358, 3)
	bypass6 := testFlow(t, "fd12:3456::6", "fd12:3456::1", 17, 53, 4)
	wan := testFlow(t, "192.168.20.5", "9.9.9.9", 17, 53, 5)
	web := testFlow(t, "192.168.10.7", "1.1.1.1", 6, 443, 6)
	web.Original.DstPort = 443
	table := &modelCT{flows: []CTFlow{own4, bypass4, own6, bypass6, wan, web}}
	cleaner := &ConntrackCleaner{Table: table}
	if e = cleaner.Evict(context.Background(), p, false); e != nil {
		t.Fatal(e)
	}
	if len(table.deleted) != 2 || len(table.flows) != 4 {
		t.Fatal("activation did not evict exactly pre-existing managed DNS bypasses")
	}
	if e = cleaner.Evict(context.Background(), p, true); e != nil {
		t.Fatal(e)
	}
	if len(table.deleted) != 4 || len(table.flows) != 2 || string(table.flows[0].ID) != string(wan.ID) || string(table.flows[1].ID) != string(web.ID) {
		t.Fatal("deactivation affected unrelated/VPN/WAN/management traffic")
	}
	if e = cleaner.Evict(context.Background(), p, true); e != nil || len(table.deleted) != 4 {
		t.Fatal("conntrack cleanup not idempotent")
	}
}
func TestP19ConntrackRejectsUnsafeIdentityAndMalformedNetlink(t *testing.T) {
	g, _, _ := gateFixture(t)
	p, _ := g.makePlan("192.168.10.1:55358")
	f := testFlow(t, "192.168.10.5", "9.9.9.9", 17, 55358, 1)
	for _, which := range []string{"missing_id", "zone", "permission"} {
		t.Run(which, func(t *testing.T) {
			bad := f
			if which == "missing_id" {
				bad.ID = nil
			}
			if which == "zone" {
				bad.Zone = 7
			}
			table := &modelCT{flows: []CTFlow{bad}}
			if which == "permission" {
				table.err = errors.New("permission denied")
			}
			if (&ConntrackCleaner{Table: table}).Evict(context.Background(), p, true) == nil {
				t.Fatal("unsafe cleanup accepted")
			}
			if len(table.deleted) != 0 {
				t.Fatal("unsafe flow deleted")
			}
		})
	}
	payload := append([]byte{f.Family, 0, 0, 0}, f.DeleteAttrs...)
	for i := 0; i < len(payload); i++ {
		_, _ = parseCTFlow(payload[:i])
	} // No panics on arbitrary truncation.
	if _, e := parseCTFlow([]byte{2, 0, 0, 0, 255, 255, 1, 0}); e == nil {
		t.Fatal("oversized attribute accepted")
	}
}

func TestP19ConntrackScopeExcludesRouterBroadcastAndUnmanaged(t *testing.T) {
	g, _, _ := gateFixture(t)
	p, _ := g.makePlan("192.168.10.1:55358")
	fixtures := []CTFlow{
		testFlow(t, "192.168.10.1", "9.9.9.9", 17, 53, 1),
		testFlow(t, "fd12:3456::1", "2606:4700:4700::1111", 6, 53, 2),
		testFlow(t, "192.168.10.5", "192.168.10.255", 17, 53, 3),
		testFlow(t, "192.168.10.5", "224.0.0.251", 17, 53, 4),
		testFlow(t, "fd12:3456::5", "ff02::fb", 17, 53, 5),
		testFlow(t, "192.168.20.5", "9.9.9.9", 17, 53, 6),
	}
	table := &modelCT{flows: fixtures}
	c := &ConntrackCleaner{Table: table}
	for _, owned := range []bool{false, true} {
		if err := c.Evict(context.Background(), p, owned); err != nil {
			t.Fatal(err)
		}
	}
	if len(table.deleted) != 0 || len(table.flows) != len(fixtures) {
		t.Fatal("out-of-scope flow deleted")
	}
}
func TestP19ConntrackAmbiguityRejectedBeforeFirewallMutation(t *testing.T) {
	g, m, k := gateFixture(t)
	f := testFlow(t, "192.168.10.5", "9.9.9.9", 17, 53, 1)
	f.Zone = 7
	k.Conntrack.Table = &modelCT{flows: []CTFlow{f}}
	if err := g.Enable(context.Background(), "192.168.10.1:55358"); err == nil {
		t.Fatal("ambiguous conntrack zone accepted")
	}
	if len(m.mutations) != 0 {
		t.Fatal("unsafe preflight mutated firewall")
	}
}
