package lite

// P15: opt-in system DNS gate. RouterForge never edits _NDM_* or Keenetic DNS
// settings. Lease-backed NAT redirects are scoped to br0 and UDP/TCP port 53.
// Kernel objects do not persist through reboot; no automatic re-activation.

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	gateChain        = "RF_DNS_LITE_GATE"
	gateSet4         = "RF_DNS_LITE4"
	gateSet6         = "RF_DNS_LITE6"
	gateInterface    = "br0"
	gateLeaseSeconds = "15"
)

type GateExec func(context.Context, string, ...string) error

type GateManager struct {
	mu         sync.Mutex
	run        GateExec
	candidates func() (string, []string, error)
	locate     func(string) (string, error)
	active     bool
	port       string
	v4prefix   string
	v6prefix   []string
	cancel     context.CancelFunc
	lastError  string
}

func gateRun(ctx context.Context, binary string, args ...string) error {
	// Only fixed executable names/arguments produced by GateManager are allowed;
	// never use shell, raw query parameters, or path-derived script commands.
	child, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	output, err := exec.CommandContext(child, binary, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w (%s)", binary, err, strings.TrimSpace(string(output)))
	}
	return nil
}
func NewGateManager() *GateManager {
	return &GateManager{run: gateRun, candidates: GateCandidates, locate: exec.LookPath}
}
func (g *GateManager) command(ctx context.Context, name string, args ...string) error {
	if g.run == nil {
		return errors.New("DNS gate executor unavailable")
	}
	return g.run(ctx, name, args...)
}

// GateCandidates obtains the actual br0 prefixes. No user-supplied interface,
// prefix, or firewall expression can reach the command runner.
func GateCandidates() (string, []string, error) {
	ifc, err := net.InterfaceByName(gateInterface)
	if err != nil || ifc.Flags&net.FlagUp == 0 {
		return "", nil, errors.New("br0 LAN interface unavailable")
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
	if v4 == "" || len(v6) == 0 {
		return "", nil, errors.New("dual-stack br0 LAN prefixes required")
	}
	// Reject intentionally ambiguous interfaces until hardware-tested.
	if len(v6) > 4 {
		return "", nil, errors.New("too many br0 IPv6 prefixes")
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
func (g *GateManager) Status() map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := "STOCK"
	if g.active {
		state = "ACTIVE"
	}
	if g.lastError != "" {
		state = "RECOVERY_REQUIRED"
	}
	return map[string]any{"state": state, "active": g.active, "port": g.port, "interface": gateInterface, "last_error": g.lastError, "lease_seconds": 15, "persisted": false}
}

// Preflight deliberately leaves kernel objects unchanged. Actual ipset timeout
// support is verified by the full enable transaction (which rolls back on error).
func (g *GateManager) Preflight(ctx context.Context, listen string) error {
	if _, err := gatePort(listen); err != nil {
		return err
	}
	if _, _, err := g.candidates(); err != nil {
		return err
	}
	for _, binary := range []string{"ipset", "iptables", "ip6tables"} {
		if _, err := g.locate(binary); err != nil {
			return fmt.Errorf("%s missing: %w", binary, err)
		}
	}
	// Existing named kernel objects might be from a crashed process or another
	// application. Never claim them without a separate ownership audit.
	for _, p := range []struct {
		cmd  string
		args []string
	}{
		{"iptables", []string{"-t", "nat", "-S", gateChain}},
		{"ip6tables", []string{"-t", "nat", "-S", gateChain}},
		{"ipset", []string{"list", gateSet4}},
		{"ipset", []string{"list", gateSet6}},
	} {
		if g.command(ctx, p.cmd, p.args...) == nil {
			return errors.New("existing DNS Gate kernel object: manual ownership audit required")
		}
	}
	return nil
}

// Explicit Enable runs under a lock. Lease entries are populated LAST, after
// both NAT families are present, preventing partial traffic capture.
func (g *GateManager) Enable(ctx context.Context, listen string) (err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active {
		return errors.New("DNS Gate is already active")
	}
	if g.lastError != "" {
		return errors.New("DNS Gate requires ownership recovery")
	}
	if err = g.Preflight(ctx, listen); err != nil {
		return err
	}
	port, _ := gatePort(listen)
	v4, v6, _ := g.candidates()
	// Verify that the already-running DNS listener accepts IPv6 connections
	// before creating any firewall objects. UDP is separately verified by E2E.
	probe, probeErr := net.DialTimeout("tcp6", "[::1]:"+port, 2*time.Second)
	if probeErr != nil {
		return fmt.Errorf("IPv6 DNS listener not available: %w", probeErr)
	}
	_ = probe.Close()
	commands := [][]string{
		{"ipset", "create", gateSet4, "hash:net", "family", "inet", "timeout", gateLeaseSeconds},
		{"ipset", "create", gateSet6, "hash:net", "family", "inet6", "timeout", gateLeaseSeconds},
		{"iptables", "-t", "nat", "-N", gateChain},
		{"ip6tables", "-t", "nat", "-N", gateChain},
		{"iptables", "-t", "nat", "-A", gateChain, "-p", "udp", "--dport", "53", "-m", "set", "--match-set", gateSet4, "src", "-j", "REDIRECT", "--to-ports", port},
		{"iptables", "-t", "nat", "-A", gateChain, "-p", "tcp", "--dport", "53", "-m", "set", "--match-set", gateSet4, "src", "-j", "REDIRECT", "--to-ports", port},
		{"ip6tables", "-t", "nat", "-A", gateChain, "-p", "udp", "--dport", "53", "-m", "set", "--match-set", gateSet6, "src", "-j", "REDIRECT", "--to-ports", port},
		{"ip6tables", "-t", "nat", "-A", gateChain, "-p", "tcp", "--dport", "53", "-m", "set", "--match-set", gateSet6, "src", "-j", "REDIRECT", "--to-ports", port},
		{"iptables", "-t", "nat", "-I", "PREROUTING", "1", "-i", gateInterface, "-j", gateChain},
		{"ip6tables", "-t", "nat", "-I", "PREROUTING", "1", "-i", gateInterface, "-j", gateChain},
	}
	for step, argv := range commands {
		if err = g.command(ctx, argv[0], argv[1:]...); err != nil {
			// No lease installed yet. On an incomplete transaction, leave inert
			// objects for explicit inspection rather than flushing an object
			// another program could have created after our preflight.
			if step > 0 {
				g.lastError = "partial Gate setup (inert, no lease): " + err.Error()
			}
			return err
		}
	}
	g.port = port
	if err = g.refreshLease(ctx, v4, v6, false); err != nil {
		// Lease may have been partially written. Flush it first; then remove
		// only our exact named objects. An error leaves the lease to expire.
		cleanup := g.removeLocked(context.Background(), false)
		if cleanup != nil {
			g.lastError = "activation lease failure; lease expires automatically; cleanup: " + cleanup.Error()
		}
		return err
	}
	g.active = true
	g.v4prefix = v4
	g.v6prefix = v6
	g.lastError = ""
	leaseCtx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	go g.renew(leaseCtx)
	return nil
}
func (g *GateManager) refreshLease(ctx context.Context, v4 string, v6 []string, exist bool) error {
	add := func(set, prefix string) error {
		args := []string{"add", set, prefix, "timeout", gateLeaseSeconds}
		if exist {
			args = append([]string{"-exist"}, args...)
		}
		return g.command(ctx, "ipset", args...)
	}
	if err := add(gateSet4, v4); err != nil {
		return err
	}
	for _, prefix := range v6 {
		if err := add(gateSet6, prefix); err != nil {
			return err
		}
	}
	return nil
}
func (g *GateManager) renew(ctx context.Context) {
	ticker := time.NewTicker(4 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.mu.Lock()
			if !g.active {
				g.mu.Unlock()
				return
			}
			// Firmware may rebuild NAT chains. If either jump disappears,
			// stop renewing instead of reporting an active global route.
			var e error
			for _, bin := range []string{"iptables", "ip6tables"} {
				if e = g.command(ctx, bin, "-t", "nat", "-C", "PREROUTING", "-i", gateInterface, "-j", gateChain); e != nil {
					break
				}
			}
			if e == nil {
				e = g.refreshLease(ctx, g.v4prefix, g.v6prefix, true)
			}
			if e != nil {
				g.lastError = "heartbeat failed; lease will expire: " + e.Error()
				g.active = false
				g.mu.Unlock()
				return
			}
			g.mu.Unlock()
		}
	}
}

func (g *GateManager) removeLocked(ctx context.Context, partial bool) error {
	// Deactivate matching first, even if ownership verification fails later.
	// Avoid touching non-owned chains/sets if the exact object is absent.
	var problems []string
	for _, set := range []string{gateSet4, gateSet6} {
		if e := g.command(ctx, "ipset", "flush", set); e != nil && !partial {
			problems = append(problems, e.Error())
		}
	}
	for _, bin := range []string{"iptables", "ip6tables"} {
		// -C prevents deleting a different rule; -D only our exact jump.
		args := []string{"-t", "nat", "-C", "PREROUTING", "-i", gateInterface, "-j", gateChain}
		if e := g.command(ctx, bin, args...); e == nil {
			args[2] = "-D"
			if e = g.command(ctx, bin, args...); e != nil {
				problems = append(problems, e.Error())
			}
		}
		set := gateSet4
		if bin == "ip6tables" {
			set = gateSet6
		}
		for _, proto := range []string{"udp", "tcp"} {
			args := []string{"-t", "nat", "-D", gateChain, "-p", proto, "--dport", "53", "-m", "set", "--match-set", set, "src", "-j", "REDIRECT", "--to-ports", g.port}
			if e := g.command(ctx, bin, args...); e != nil && !partial {
				problems = append(problems, e.Error())
			}
		}
		// -X succeeds only for an empty chain. Never flush an unexpected
		// third-party rule, even if placed inside our named chain.
		if e := g.command(ctx, bin, "-t", "nat", "-X", gateChain); e != nil && !partial {
			problems = append(problems, e.Error())
		}
	}
	for _, set := range []string{gateSet4, gateSet6} {
		if e := g.command(ctx, "ipset", "destroy", set); e != nil && !partial {
			problems = append(problems, e.Error())
		}
	}
	if len(problems) != 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}
func (g *GateManager) Disable(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	if !g.active && g.lastError == "" {
		return nil
	}
	err := g.removeLocked(ctx, false)
	g.active = false
	g.port = ""
	if err != nil {
		g.lastError = err.Error()
		return err
	}
	g.lastError = ""
	return nil
}
func (g *GateManager) Close() error {
	// With a crashed daemon the lease expires; on graceful shutdown we flush
	// and remove our own firewall objects before the DNS listener is closed.
	return g.Disable(context.Background())
}
