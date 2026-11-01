package lite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const (
	gateChain     = "RF_DNS_LITE_GATE"
	gateSet4      = "RF_DNS_LITE4"
	gateSet6      = "RF_DNS_LITE6"
	gateInterface = "br0"
)

type GateManager struct {
	mu          sync.Mutex
	op          sync.Mutex
	kernel      GateKernel
	candidates  func() (string, []string, error)
	planFactory func(string) (GatePlan, error)
	locate      func(string) (string, error)
	probe       func(context.Context, string, GatePlan) error
	plan        GatePlan
	listen      string
	desired     bool
	active      bool
	paused      bool
	state       string
	leaseUntil  time.Time
	verifiedAt  time.Time
	lastError   string
	failures    int
	cancel      context.CancelFunc
	notify      chan struct{}
	now         func() time.Time
}

func NewGateManager() *GateManager {
	x := &XTables{Exec: systemFirewallIO}
	return &GateManager{kernel: &LinuxGate{Tables: x, Conntrack: newSystemConntrack()}, candidates: GateCandidates, locate: exec.LookPath, probe: probeGateDNS, state: "STOCK", notify: make(chan struct{}, 1), now: time.Now}
}
func (g *GateManager) SetOwnershipPath(path string) {
	if k, ok := g.kernel.(*LinuxGate); ok {
		k.OwnershipPath = path
	}
}
func (g *GateManager) Status() map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.state
	if state == "" {
		state = "STOCK"
	}
	active := g.active && g.now().Before(g.leaseUntil)
	if g.active && !active {
		state = "DEGRADED"
	}
	return map[string]any{"state": state, "active": active, "desired": g.desired, "port": g.plan.Port, "interface": gateInterface, "last_error": g.lastError, "lease_seconds": controllerLease, "lease_until": g.leaseUntil.UTC().Format(time.RFC3339Nano), "verified_at": g.verifiedAt.UTC().Format(time.RFC3339Nano), "persisted": true, "recovery_attempts": g.failures, "families": g.plan.Families, "hardware_e2e": "PENDING"}
}
func (g *GateManager) acquire(ctx context.Context) error {
	for !g.op.TryLock() {
		if e := waitContext(ctx, 10*time.Millisecond); e != nil {
			return e
		}
	}
	return nil
}
func (g *GateManager) makePlan(listen string) (GatePlan, error) {
	if g.planFactory != nil {
		p, e := g.planFactory(listen)
		if e != nil {
			return p, e
		}
		return p, validateGatePlan(p)
	}
	port, err := gatePort(listen)
	if err != nil {
		return GatePlan{}, err
	}
	v4, v6, err := g.candidates()
	if err != nil {
		return GatePlan{}, err
	}
	host, _, _ := net.SplitHostPort(listen)
	original, _, err := net.ParseCIDR(v4)
	if err != nil {
		return GatePlan{}, err
	}
	if net.ParseIP(host) == nil || !original.Equal(net.ParseIP(host)) {
		return GatePlan{}, errors.New("DNS listener must bind the managed bridge IPv4 address")
	}
	p := GatePlan{Version: 1, Interface: gateInterface, Port: port}
	for _, item := range []struct {
		binary, set, family string
		prefixes            []string
	}{{"iptables", gateSet4, "inet", []string{v4}}, {"ip6tables", gateSet6, "inet6", v6}} {
		if len(item.prefixes) == 0 {
			continue
		}
		f := GateFamily{Binary: item.binary, Set: item.set, Family: item.family}
		for _, raw := range item.prefixes {
			ip, n, e := net.ParseCIDR(raw)
			if e != nil || ip.IsMulticast() || ip.IsUnspecified() || ip.IsLoopback() {
				return p, errors.New("invalid managed bridge prefix")
			}
			if item.family == "inet" && !ip.IsPrivate() {
				return p, errors.New("managed IPv4 bridge must use a private prefix")
			}
			ones, bits := n.Mask.Size()
			if ones == 0 || (bits == 32 && ones < 8) || (bits == 128 && ones < 32) {
				return p, errors.New("managed prefix is too broad")
			}
			f.Prefixes = append(f.Prefixes, n.String())
			p.RouterAddresses = append(p.RouterAddresses, ip.String())
		}
		p.Families = append(p.Families, f)
	}
	return p, nil
}
func (g *GateManager) Preflight(ctx context.Context, listen string) error {
	p, e := g.makePlan(listen)
	if e != nil {
		return e
	}
	for _, name := range []string{"ipset", "iptables"} {
		if _, e = g.locate(name); e != nil {
			return e
		}
	}
	if len(p.Families) > 1 {
		if _, e = g.locate("ip6tables"); e != nil {
			return fmt.Errorf("IPv6 LAN requires ip6tables: %w", e)
		}
	}
	if g.probe == nil {
		return errors.New("DNS readiness probe unavailable")
	}
	if audit, ok := g.kernel.(interface {
		Audit(context.Context, GatePlan) error
	}); ok {
		if e := audit.Audit(ctx, p); e != nil {
			return e
		}
	}
	g.mu.Lock()
	probe := g.probe
	g.mu.Unlock()
	return probe(ctx, listen, p)
}
func (g *GateManager) Enable(ctx context.Context, listen string) error {
	if e := g.acquire(ctx); e != nil {
		return e
	}
	defer g.op.Unlock()
	p, e := g.makePlan(listen)
	if e != nil {
		g.failed(e)
		return e
	}
	if e = g.Preflight(ctx, listen); e != nil {
		g.failed(e)
		return e
	}
	until, e := g.kernel.Ensure(ctx, p)
	if e != nil {
		g.failed(e)
		return e
	}
	g.mu.Lock()
	g.plan = p
	g.listen = listen
	g.desired = true
	g.paused = false
	g.active = true
	g.state = "ACTIVE"
	g.lastError = ""
	g.failures = 0
	g.leaseUntil = until
	g.verifiedAt = g.now()
	g.mu.Unlock()
	g.startWorker()
	return nil
}
func (g *GateManager) failed(e error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active = false
	g.failures++
	g.lastError = e.Error()
	g.state = "DEGRADED"
	var conflictErr *GateConflict
	if errors.As(e, &conflictErr) {
		g.state = "CONFLICT"
		g.failures = 3
	} else if g.failures >= 3 {
		g.state = "RECOVERY_REQUIRED"
	}
}
func (g *GateManager) startWorker() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	go func() {
		t := time.NewTicker(4 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-g.notify:
			}
			child, c := context.WithTimeout(ctx, 8*time.Second)
			_ = g.Reconcile(child, false)
			c()
		}
	}()
}
func (g *GateManager) Notify() {
	g.mu.Lock()
	desired := g.desired && !g.paused
	g.mu.Unlock()
	if desired {
		select {
		case g.notify <- struct{}{}:
		default:
		}
	}
}
func (g *GateManager) Reconcile(ctx context.Context, explicit bool) error {
	if e := g.acquire(ctx); e != nil {
		return e
	}
	defer g.op.Unlock()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	g.mu.Lock()
	desired := g.desired && !g.paused
	p := g.plan
	attempts := g.failures
	if explicit {
		g.failures = 0
		attempts = 0
	}
	g.mu.Unlock()
	if !desired {
		return nil
	}
	if attempts >= 3 {
		return errors.New("DNS controller recovery budget exhausted; explicit retry required")
	}
	// Re-discover addresses on every round, but only on the fixed managed bridge.
	listen := ""
	// The readiness probe uses the actual configured bind address, not the CIDR.
	g.mu.Lock()
	probeListen := g.listen
	g.mu.Unlock()
	if probeListen != "" {
		listen = probeListen
	}
	next, e := g.makePlan(listen)
	if e == nil {
		g.mu.Lock()
		probe := g.probe
		g.mu.Unlock()
		e = probe(ctx, listen, next)
	}
	if e != nil && len(p.Families) > 0 {
		if d, ok := g.kernel.(interface {
			Deactivate(context.Context, GatePlan) error
		}); ok {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			deactivate := d.Deactivate(cleanup, p)
			cancel()
			if deactivate != nil {
				e = fmt.Errorf("readiness failed: %v; deactivation failed: %w", e, deactivate)
			}
		}
	}
	var until time.Time
	if e == nil {
		until, e = g.kernel.Ensure(ctx, next)
	}
	if e != nil {
		g.failed(e)
		g.mu.Lock()
		stop := g.failures >= 3
		g.mu.Unlock()
		if stop {
			cleanup, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			_ = g.kernel.Remove(cleanup, p, false)
			cancel()
		}
		return e
	}
	g.mu.Lock()
	g.plan = next
	g.active = true
	g.state = "ACTIVE"
	g.lastError = ""
	g.failures = 0
	g.leaseUntil = until
	g.verifiedAt = g.now()
	g.mu.Unlock()
	return nil
}
func (g *GateManager) Suspend(ctx context.Context) error {
	if e := g.acquire(ctx); e != nil {
		return e
	}
	defer g.op.Unlock()
	g.mu.Lock()
	g.paused = true
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	p := g.plan
	g.active = false
	g.mu.Unlock()
	if len(p.Families) == 0 {
		if k, ok := g.kernel.(*LinuxGate); ok {
			old, e := k.receipt()
			if os.IsNotExist(e) {
				return nil
			}
			if e != nil {
				return e
			}
			p = old
		} else {
			return nil
		}
	}
	e := g.kernel.Remove(ctx, p, false)
	g.mu.Lock()
	defer g.mu.Unlock()
	if e != nil {
		g.state = "RECOVERY_REQUIRED"
		g.lastError = e.Error()
		return e
	}
	g.state = "STOCK"
	g.lastError = ""
	g.leaseUntil = time.Time{}
	return nil
}
func (g *GateManager) Disable(ctx context.Context) error {
	g.mu.Lock()
	g.desired = false
	p := g.plan
	g.mu.Unlock()
	err := g.Suspend(ctx)
	var c *GateConflict
	if errors.As(err, &c) {
		if k, ok := g.kernel.(*LinuxGate); ok {
			err = k.Remove(ctx, p, true)
			if err == nil {
				g.mu.Lock()
				g.lastError = ""
				g.state = "STOCK"
				g.mu.Unlock()
			}
		}
	}
	return err
}
func (g *GateManager) Configure(listen string) error {
	p, err := g.makePlan(listen)
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.plan = p
	g.listen = listen
	g.mu.Unlock()
	return nil
}
func (g *GateManager) Restore(ctx context.Context, listen string) error {
	g.mu.Lock()
	g.desired = true
	g.paused = false
	g.listen = listen
	g.mu.Unlock()
	err := g.Enable(ctx, listen)
	if err != nil {
		g.startWorker()
	}
	return err
}
func (g *GateManager) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return g.Suspend(ctx)
}

func probeGateDNS(ctx context.Context, listen string, p GatePlan) error {
	return probeGateDNSWithTrace(ctx, listen, p, nil)
}
func probeGateDNSWithTrace(ctx context.Context, listen string, p GatePlan, observed func(string) bool) error {
	var nonce [12]byte
	if _, e := rand.Read(nonce[:]); e != nil {
		return e
	}
	for _, f := range p.Families {
		destination := listen
		if f.Family == "inet6" {
			destination = net.JoinHostPort("::1", p.Port)
		}
		for _, network := range []string{"udp", "tcp"} {
			q := new(dns.Msg)
			q.SetQuestion("rf-ready-"+hex.EncodeToString(nonce[:])+"-"+network+".invalid.", dns.TypeTXT)
			reply, _, e := (&dns.Client{Net: network, Timeout: 1500 * time.Millisecond}).ExchangeContext(ctx, q, destination)
			if e != nil {
				return fmt.Errorf("%s %s readiness: %w", f.Family, network, e)
			}
			if reply == nil || !reply.Response || reply.Id != q.Id || len(reply.Question) != 1 || reply.Question[0] != q.Question[0] || reply.Truncated || (reply.Rcode != dns.RcodeSuccess && reply.Rcode != dns.RcodeNameError) {
				return fmt.Errorf("%s %s DNS readiness response invalid", f.Family, network)
			}
			if observed != nil && !observed(q.Question[0].Name) {
				return errors.New("DNS answer has no attribution to this RouterForge engine")
			}
		}
	}
	return nil
}

func GateCandidates() (string, []string, error) {
	ifc, err := net.InterfaceByName(gateInterface)
	if err != nil || ifc.Flags&net.FlagUp == 0 {
		return "", nil, errors.New("br0 LAN interface unavailable")
	}
	if fi, e := os.Stat("/sys/class/net/" + gateInterface + "/bridge"); e != nil || !fi.IsDir() {
		return "", nil, errors.New("managed br0 is not a kernel LAN bridge")
	}
	addrs, err := ifc.Addrs()
	if err != nil {
		return "", nil, err
	}
	v4 := ""
	v6 := []string{}
	for _, item := range addrs {
		ipnet, ok := item.(*net.IPNet)
		if !ok {
			continue
		}
		if ipnet.IP.To4() != nil {
			if v4 != "" {
				return "", nil, errors.New("multiple br0 IPv4 prefixes require explicit audit")
			}
			v4 = ipnet.String()
		} else if ipnet.IP.To16() != nil && !ipnet.IP.IsLoopback() {
			v6 = append(v6, ipnet.String())
		}
	}
	if v4 == "" {
		return "", nil, errors.New("br0 IPv4 LAN prefix required")
	}
	// Reject intentionally ambiguous interfaces until hardware-tested.
	if len(v6) > 4 {
		return "", nil, errors.New("too many br0 IPv6 prefixes")
	}
	interfaces, e := net.Interfaces()
	if e != nil {
		return "", nil, e
	}
	managed := append([]string{v4}, v6...)
	for _, other := range interfaces {
		if other.Name == gateInterface || other.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, e := other.Addrs()
		if e != nil {
			return "", nil, e
		}
		for _, address := range addresses {
			otherIP, otherNet, e := net.ParseCIDR(address.String())
			if e != nil {
				continue
			}
			for _, prefix := range managed {
				ownIP, ownNet, _ := net.ParseCIDR(prefix)
				if ownIP.IsLinkLocalUnicast() && otherIP.IsLinkLocalUnicast() {
					if ownIP.Equal(otherIP) {
						return "", nil, errors.New("ambiguous scoped router IPv6 address")
					}
					continue
				}
				if ownNet.Contains(otherIP) || otherNet.Contains(ownIP) {
					return "", nil, fmt.Errorf("managed LAN prefix overlaps %s; conntrack scope is ambiguous", other.Name)
				}
			}
		}
	}
	return v4, v6, nil
}
func gatePort(listen string) (string, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil || net.ParseIP(host) == nil {
		return "", errors.New("DNS listener must use an explicit IP:port")
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1024 || n > 65535 {
		return "", errors.New("DNS Gate requires an unprivileged listener port")
	}
	return port, nil
}

func gateMissingObject(err error, output string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error() + " " + output)
	return strings.Contains(msg, "the set with the given name does not exist") ||
		strings.Contains(msg, "no chain/target/match by that name") ||
		strings.Contains(msg, "doesn't exist")
}

func (g *GateManager) SetProbe(probe func(context.Context, string, GatePlan) error) {
	g.mu.Lock()
	g.probe = probe
	g.mu.Unlock()
}

func (g *GateManager) Quiesce(ctx context.Context) error {
	g.mu.Lock()
	p := g.plan
	g.active = false
	g.paused = true
	g.mu.Unlock()
	if len(p.Families) == 0 {
		if k, ok := g.kernel.(*LinuxGate); ok {
			var e error
			p, e = k.receipt()
			if os.IsNotExist(e) {
				return nil
			}
			if e != nil {
				return e
			}
		} else {
			return nil
		}
	}
	if d, ok := g.kernel.(interface {
		Deactivate(context.Context, GatePlan) error
	}); ok {
		return d.Deactivate(ctx, p)
	}
	return g.kernel.Remove(ctx, p, false)
}
