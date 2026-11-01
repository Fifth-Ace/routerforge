package lite

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
)

// Netfilter UAPI attributes. Only copied kernel tuples with an exact CTA_ID
// may be deleted; a delete message without a tuple would flush the table.
type ctTuple struct {
	Src, Dst         net.IP
	Proto            byte
	SrcPort, DstPort uint16
}
type CTFlow struct {
	Family          byte
	Original, Reply ctTuple
	ID              []byte
	Zone            uint16
	DeleteAttrs     []byte
}
type CTTable interface {
	Dump(context.Context) ([]CTFlow, error)
	Delete(context.Context, CTFlow) error
}
type ConntrackCleaner struct{ Table CTTable }

func (c *ConntrackCleaner) Check(ctx context.Context) error {
	if c == nil || c.Table == nil {
		return errors.New("conntrack backend unavailable")
	}
	_, err := c.Table.Dump(ctx)
	return err
}
func validateManagedFlow(p GatePlan, f CTFlow) error {
	if f.Zone != 0 {
		return errors.New("managed DNS flow uses an ambiguous conntrack zone")
	}
	if len(f.ID) != 4 || len(f.DeleteAttrs) == 0 {
		return errors.New("conntrack flow has no safe exact identity")
	}
	port, _ := strconv.Atoi(p.Port)
	if f.Original.Src.IsLinkLocalUnicast() && f.Reply.SrcPort != uint16(port) {
		for _, address := range p.RouterAddresses {
			if net.ParseIP(address).Equal(f.Original.Dst) {
				return nil
			}
		}
		return errors.New("ambiguous link-local DNS conntrack scope")
	}
	return nil
}

// CheckPlan rejects ambiguous managed tuples before any firewall mutation.
func (c *ConntrackCleaner) CheckPlan(ctx context.Context, p GatePlan) error {
	if c == nil || c.Table == nil {
		return errors.New("conntrack backend unavailable")
	}
	flows, err := c.Table.Dump(ctx)
	if err != nil {
		return err
	}
	for _, f := range flows {
		if managedFlow(p, f) {
			if err := validateManagedFlow(p, f); err != nil {
				return err
			}
		}
	}
	return nil
}
func managedFlow(p GatePlan, f CTFlow) bool {
	for _, own := range p.RouterAddresses {
		if net.ParseIP(own).Equal(f.Original.Src) {
			return false
		}
	}
	for _, family := range p.Families {
		for _, prefix := range family.Prefixes {
			if broadcast := prefixBroadcast(prefix); broadcast != nil && broadcast.Equal(f.Original.Dst) {
				return false
			}
		}
	}
	if f.Original.Dst.IsMulticast() || f.Original.Dst.Equal(net.IPv4bcast) {
		return false
	}
	if f.Original.DstPort != 53 || (f.Original.Proto != 6 && f.Original.Proto != 17) {
		return false
	}
	for _, family := range p.Families {
		if (f.Family == 2) != (family.Family == "inet") {
			continue
		}
		for _, raw := range family.Prefixes {
			_, n, _ := net.ParseCIDR(raw)
			if n != nil && n.Contains(f.Original.Src) {
				return true
			}
		}
	}
	return false
}
func (c *ConntrackCleaner) Evict(ctx context.Context, p GatePlan, ownedOnly bool) error {
	return c.evict(ctx, p, ownedOnly, false)
}
func (c *ConntrackCleaner) evictLegacy(ctx context.Context, p GatePlan) error {
	return c.evict(ctx, p, true, true)
}
func (c *ConntrackCleaner) evict(ctx context.Context, p GatePlan, ownedOnly, rejectScoped bool) error {
	if c == nil || c.Table == nil {
		return errors.New("conntrack backend unavailable")
	}
	port, _ := strconv.Atoi(p.Port)
	matches := func(f CTFlow) bool { return managedFlow(p, f) && ((f.Reply.SrcPort == uint16(port)) == ownedOnly) }
	// A second pass handles dump races. Activation deletes pre-existing bypass
	// mappings, preserving new RouterForge flows; deactivation deletes only our
	// high-port redirects, preserving new stock mappings and every other flow.
	for pass := 0; pass < 2; pass++ {
		flows, err := c.Table.Dump(ctx)
		if err != nil {
			return err
		}
		pending := 0
		for _, f := range flows {
			if !matches(f) {
				continue
			}
			pending++
			if rejectScoped && (f.Original.Src.IsLinkLocalUnicast() || f.Original.Dst.IsLinkLocalUnicast() || f.Reply.Src.IsLinkLocalUnicast()) {
				return conflict("legacy link-local conntrack appeared during handoff; interface ownership unproven")
			}
			if err := validateManagedFlow(p, f); err != nil {
				return err
			}
			if err = c.Table.Delete(ctx, f); err != nil {
				return err
			}
		}
		if pending == 0 {
			return nil
		}
	}
	flows, err := c.Table.Dump(ctx)
	if err != nil {
		return err
	}
	for _, f := range flows {
		if matches(f) {
			return errors.New("conntrack mappings changed beyond reconciliation budget")
		}
	}
	return nil
}
func align4(n int) int { return (n + 3) &^ 3 }
func ctAttributes(b []byte) (map[uint16][]byte, map[uint16][]byte, error) {
	values := map[uint16][]byte{}
	raw := map[uint16][]byte{}
	for len(b) > 0 {
		if len(b) < 4 {
			return nil, nil, errors.New("truncated conntrack attribute")
		}
		n := int(binary.NativeEndian.Uint16(b[:2]))
		kind := binary.NativeEndian.Uint16(b[2:4]) & 0x3fff
		if n < 4 || n > len(b) || align4(n) > len(b) {
			return nil, nil, errors.New("invalid conntrack attribute length")
		}
		if _, dup := values[kind]; dup {
			return nil, nil, errors.New("duplicate conntrack attribute")
		}
		values[kind] = b[4:n]
		raw[kind] = b[:align4(n)]
		b = b[align4(n):]
	}
	return values, raw, nil
}
func parseCTTuple(b []byte) (ctTuple, error) {
	var t ctTuple
	a, _, err := ctAttributes(b)
	if err != nil {
		return t, err
	}
	ip, _, err := ctAttributes(a[1])
	if err != nil {
		return t, err
	}
	proto, _, err := ctAttributes(a[2])
	if err != nil {
		return t, err
	}
	src, dst := ip[1], ip[2]
	if len(ip[3]) > 0 {
		src, dst = ip[3], ip[4]
	}
	if len(src) != 4 && len(src) != 16 || len(dst) != len(src) || len(proto[1]) != 1 {
		return t, errors.New("invalid conntrack tuple")
	}
	t.Src = append(net.IP{}, src...)
	t.Dst = append(net.IP{}, dst...)
	t.Proto = proto[1][0]
	if t.Proto == 6 || t.Proto == 17 {
		if len(proto[2]) != 2 || len(proto[3]) != 2 {
			return t, errors.New("invalid conntrack ports")
		}
		t.SrcPort = binary.BigEndian.Uint16(proto[2])
		t.DstPort = binary.BigEndian.Uint16(proto[3])
	}
	return t, nil
}
func parseCTFlow(payload []byte) (CTFlow, error) {
	var f CTFlow
	if len(payload) < 4 {
		return f, errors.New("short conntrack netfilter message")
	}
	f.Family = payload[0]
	if f.Family != 2 && f.Family != 10 {
		return f, errors.New("unsupported conntrack address family")
	}
	a, raw, err := ctAttributes(payload[4:])
	if err != nil {
		return f, err
	}
	f.Original, err = parseCTTuple(a[1])
	if err != nil {
		return f, err
	}
	f.Reply, err = parseCTTuple(a[2])
	if err != nil {
		return f, err
	}
	f.ID = append([]byte{}, a[12]...)
	if len(a[18]) > 0 {
		if len(a[18]) != 2 {
			return f, errors.New("invalid conntrack zone")
		}
		f.Zone = binary.BigEndian.Uint16(a[18])
	}
	if len(f.ID) != 4 {
		return f, errors.New("conntrack dump lacks CTA_ID")
	}
	f.DeleteAttrs = append(f.DeleteAttrs, raw[1]...)
	f.DeleteAttrs = append(f.DeleteAttrs, raw[2]...)
	f.DeleteAttrs = append(f.DeleteAttrs, raw[12]...)
	f.DeleteAttrs = append(f.DeleteAttrs, raw[18]...)
	if len(f.DeleteAttrs) == 0 {
		return f, fmt.Errorf("unsafe conntrack delete identity")
	}
	return f, nil
}
