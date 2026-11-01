package lite

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type modelSet struct {
	family  string
	members map[string]time.Time
}
type modelKernel struct {
	filters   map[string]map[string][][]string
	mu        sync.Mutex
	chains    map[string]map[string][][]string
	mangle    map[string]map[string][][]string
	sets      map[string]*modelSet
	calls     []string
	mutations []string
	locks     int
	fail      string
	now       time.Time
}

func newModel() *modelKernel {
	m := &modelKernel{chains: map[string]map[string][][]string{}, sets: map[string]*modelSet{}, now: time.Now()}
	m.mangle = map[string]map[string][][]string{}
	m.filters = map[string]map[string][][]string{}
	for _, b := range []string{"iptables", "ip6tables"} {
		m.mangle[b] = map[string][][]string{"PREROUTING": {}}
		m.filters[b] = map[string][][]string{"INPUT": {}}
		m.chains[b] = map[string][][]string{"PREROUTING": {{"-A", "PREROUTING", "-j", "_NDM_DNS_REDIRECT"}}, "_NDM_DNS_REDIRECT": {}}
	}
	return m
}
func (m *modelKernel) exec(_ context.Context, b string, a ...string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, b+" "+strings.Join(a, " "))
	if b != "ipset" {
		if len(a) < 2 || a[0] != "-w" || a[1] != "1" {
			return "", fmt.Errorf("missing bounded xtables wait: %v", a)
		}
		a = a[2:]
		if m.locks != 0 {
			if m.locks > 0 {
				m.locks--
			}
			return "Another app is currently holding the xtables lock", errors.New("exit status 4")
		}
	}
	command := b + " " + strings.Join(a, " ")
	if m.fail != "" && strings.Contains(command, m.fail) {
		return "permission denied", errors.New("exit status 1")
	}
	if b == "ipset" {
		if a[0] == "-exist" {
			a = a[1:]
		}
		if len(a) < 2 {
			return "", errors.New("bad ipset syntax")
		}
		name := a[1]
		set := m.sets[name]
		switch a[0] {
		case "save":
			if set == nil {
				return "The set with the given name does not exist", errors.New("exit status 1")
			}
			out := "create " + name + " hash:net family " + set.family + " timeout 60\n"
			keys := []string{}
			for k := range set.members {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, key := range keys {
				ttl := int(set.members[key].Sub(m.now).Seconds())
				if ttl > 0 {
					out += fmt.Sprintf("add %s %s timeout %d\n", name, key, ttl)
				}
			}
			return out, nil
		case "create":
			if set != nil {
				return "", errors.New("set exists")
			}
			m.sets[name] = &modelSet{family: argValue(a, "family"), members: map[string]time.Time{}}
		case "add":
			if set == nil {
				return "missing set", errors.New("missing set")
			}
			n, _ := strconv.Atoi(argValue(a, "timeout"))
			set.members[a[2]] = m.now.Add(time.Duration(n) * time.Second)
		case "flush":
			if set == nil {
				return "The set with the given name does not exist", errors.New("missing set")
			}
			set.members = map[string]time.Time{}
		case "destroy":
			if set == nil {
				return "The set with the given name does not exist", errors.New("missing set")
			}
			for _, chains := range m.chains {
				for _, rs := range chains {
					for _, r := range rs {
						if argValue(r, "--match-set") == name {
							return "set in use", errors.New("set in use")
						}
					}
				}
			}
			for _, chains := range m.filters {
				for _, rs := range chains {
					for _, r := range rs {
						if argValue(r, "--match-set") == name {
							return "set in use", errors.New("filter set reference")
						}
					}
				}
			}
			delete(m.sets, name)
		default:
			return "", fmt.Errorf("unmodeled ipset operation %v", a)
		}
	} else {
		if len(a) >= 3 && a[0] == "-t" && a[1] == "mangle" && a[2] == "-S" {
			out := "-P PREROUTING ACCEPT\n"
			for _, rs := range m.mangle[b] {
				for _, r := range rs {
					out += ruleString(r) + "\n"
				}
			}
			return out, nil
		}
		if len(a) < 3 || a[0] != "-t" || (a[1] != "nat" && a[1] != "filter") {
			return "", fmt.Errorf("unsafe table %v", a)
		}
		table := m.chains[b]
		filter := a[1] == "filter"
		if filter {
			table = m.filters[b]
		}
		a = a[2:]
		action := a[0]
		if action == "-S" {
			out := "-P PREROUTING ACCEPT\n"
			if filter {
				out = "-P INPUT DROP\n"
			}
			names := []string{}
			for name := range table {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				if name != "PREROUTING" {
					out += "-N " + name + "\n"
				}
				for _, r := range table[name] {
					out += ruleString(r) + "\n"
				}
			}
			return out, nil
		}
		name := a[1]
		_, exists := table[name]
		switch action {
		case "-N":
			if exists {
				return "", errors.New("chain exists")
			}
			table[name] = nil
		case "-A", "-I":
			if !exists {
				return "No chain/target/match by that name", errors.New("missing chain")
			}
			r := append([]string{}, a...)
			r[0] = "-A"
			if action == "-I" {
				if a[2] != "1" {
					return "", errors.New("only head insertion allowed")
				}
				r = append(r[:2], r[3:]...)
				table[name] = append([][]string{r}, table[name]...)
			} else {
				table[name] = append(table[name], r)
			}
		case "-D":
			wanted := append([]string{}, a...)
			wanted[0] = "-A"
			found := false
			for i, r := range table[name] {
				if ruleString(r) == ruleString(wanted) {
					table[name] = append(table[name][:i], table[name][i+1:]...)
					found = true
					break
				}
			}
			if !found {
				return "Bad rule", errors.New("rule missing")
			}
		case "-F":
			if name != gateChain {
				return "", errors.New("foreign flush forbidden")
			}
			table[name] = nil
		case "-X":
			if !exists {
				return "No chain/target/match by that name", errors.New("missing chain")
			}
			if len(table[name]) != 0 {
				return "chain not empty", errors.New("chain not empty")
			}
			for _, rs := range table {
				for _, r := range rs {
					if argValue(r, "-j") == name {
						return "chain referenced", errors.New("chain referenced")
					}
				}
			}
			delete(table, name)
		default:
			return "", fmt.Errorf("unmodeled iptables operation %v", a)
		}
	}
	m.mutations = append(m.mutations, command)
	return "", nil
}

// The packet model interprets the generated chain, interface, source set,
// destination and protocol. These are fresh-flow NAT simulations, not hardware.
func (m *modelKernel) packet(family, iface, source, destination, proto string, port int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	bin := "iptables"
	if family == "inet6" {
		bin = "ip6tables"
	}
	var walk func(string) string
	walk = func(chain string) string {
		if chain == "_NDM_DNS_REDIRECT" {
			return "STOCK"
		}
		rules := m.chains[bin][chain]
		if chain == "INPUT" {
			rules = m.filters[bin][chain]
		}
		for _, r := range rules {
			if i := argValue(r, "-i"); i != "" && i != iface {
				continue
			}
			if p := argValue(r, "-p"); p != "" && p != proto {
				continue
			}
			if p := argValue(r, "--dport"); p != "" && p != strconv.Itoa(port) {
				continue
			}
			if d := argValue(r, "-d"); d != "" {
				_, n, _ := net.ParseCIDR(d)
				if n == nil || !n.Contains(net.ParseIP(destination)) {
					continue
				}
			}
			if set := argValue(r, "--match-set"); set != "" {
				s := m.sets[set]
				match := false
				if s != nil {
					for raw, until := range s.members {
						_, n, _ := net.ParseCIDR(raw)
						match = match || until.After(m.now) && n.Contains(net.ParseIP(source))
					}
				}
				if !match {
					continue
				}
			}
			target := argValue(r, "-j")
			if target == "ACCEPT" {
				return "ACCEPT"
			}
			if target == "DROP" {
				return "DROP"
			}
			if target == "RETURN" {
				return ""
			}
			if target == "REDIRECT" {
				return "ROUTERFORGE:" + argValue(r, "--to-ports")
			}
			if v := walk(target); v != "" {
				return v
			}
		}
		return ""
	}
	result := walk("PREROUTING")
	if strings.HasPrefix(result, "ROUTERFORGE:") {
		port, _ = strconv.Atoi(strings.TrimPrefix(result, "ROUTERFORGE:"))
		if walk("INPUT") != "ACCEPT" {
			return "INPUT_DROP"
		}
	}
	return result
}
func gateFixture(t *testing.T) (*GateManager, *modelKernel, *LinuxGate) {
	t.Helper()
	m := newModel()
	k := &LinuxGate{Tables: &XTables{Exec: m.exec, Delay: func(context.Context, time.Duration) error { return nil }}, OwnershipPath: filepath.Join(t.TempDir(), "owned.json"), Conntrack: &ConntrackCleaner{Table: &modelCT{}}}
	g := NewGateManager()
	g.kernel = k
	g.candidates = func() (string, []string, error) {
		return "192.168.10.1/24", []string{"fe80::1/64", "fd12:3456::1/64"}, nil
	}
	g.locate = func(string) (string, error) { return "/injected", nil }
	g.probe = func(context.Context, string, GatePlan) error { return nil }
	t.Cleanup(func() { _ = g.Close() })
	return g, m, k
}
func TestP19GateDualStackFreshFlowScopeAndIdempotence(t *testing.T) {
	g, m, _ := gateFixture(t)
	if g.Status()["active"] != false {
		t.Fatal("fresh Gate activated itself")
	}
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	for _, family := range []string{"inet", "inet6"} {
		source := "192.168.10.55"
		destinations := []string{"192.168.10.1", "9.9.9.9"}
		if family == "inet6" {
			source = "fd12:3456::55"
			destinations = []string{"fd12:3456::1", "2606:4700:4700::1111"}
		}
		for _, proto := range []string{"udp", "tcp"} {
			for _, destination := range destinations {
				if got := m.packet(family, "br0", source, destination, proto, 53); got != "ROUTERFORGE:55358" {
					t.Fatalf("%s %s %s -> %s", family, proto, destination, got)
				}
			}
		}
		for _, tc := range []struct {
			iface, proto string
			port         int
		}{{"eth3", "udp", 53}, {"nwg0", "tcp", 53}, {"", "udp", 53}, {"br0", "udp", 5353}, {"br0", "tcp", 443}} {
			if got := m.packet(family, tc.iface, source, destinations[0], tc.proto, tc.port); strings.HasPrefix(got, "ROUTERFORGE") {
				t.Fatalf("captured unmanaged traffic: %+v", tc)
			}
		}
	}
	for _, d := range []string{"224.0.0.251", "255.255.255.255"} {
		if strings.HasPrefix(m.packet("inet", "br0", "192.168.10.55", d, "udp", 53), "ROUTERFORGE") {
			t.Fatal("multicast/broadcast captured")
		}
	}
	if strings.HasPrefix(m.packet("inet6", "br0", "fd12:3456::55", "ff02::fb", "udp", 53), "ROUTERFORGE") {
		t.Fatal("IPv6 multicast captured")
	}
	m.mu.Lock()
	before := len(m.mutations)
	m.mu.Unlock()
	for i := 0; i < 30; i++ {
		if e := g.Reconcile(context.Background(), false); e != nil {
			t.Fatal(e)
		}
	}
	m.mu.Lock()
	for _, c := range m.mutations[before:] {
		if strings.HasPrefix(c, "iptables") || strings.HasPrefix(c, "ip6tables") {
			t.Fatalf("correct firewall was needlessly mutated: %s", c)
		}
	}
	m.mu.Unlock()
	if e := g.Disable(context.Background()); e != nil {
		t.Fatal(e)
	}
	if got := m.packet("inet", "br0", "192.168.10.55", "9.9.9.9", "udp", 53); got != "STOCK" {
		t.Fatal("stock fallback absent", got)
	}
	if e := g.Disable(context.Background()); e != nil {
		t.Fatal("disable not idempotent", e)
	}
}
func TestP19TransientLockRecoveryLeaseExpiryAndNonLockErrors(t *testing.T) {
	g, m, _ := gateFixture(t)
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	m.locks = 2
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e != nil {
		t.Fatal("bounded contention recovery failed", e)
	}
	m.mu.Lock()
	m.locks = -1
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e == nil || g.Status()["state"] != "DEGRADED" {
		t.Fatal("transient error is not DEGRADED")
	}
	g.mu.Lock()
	g.leaseUntil = time.Now().Add(-time.Second)
	g.mu.Unlock()
	if g.Status()["active"] != false {
		t.Fatal("expired lease reported ACTIVE")
	}
	for i := 0; i < 2; i++ {
		_ = g.Reconcile(context.Background(), false)
	}
	if g.Status()["state"] != "RECOVERY_REQUIRED" {
		t.Fatal("unbounded recovery")
	}
	m.mu.Lock()
	n := len(m.calls)
	m.locks = 0
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e == nil {
		t.Fatal("exhausted budget retried implicitly")
	}
	m.mu.Lock()
	after := len(m.calls)
	m.mu.Unlock()
	if after != n {
		t.Fatal("exhausted recovery executed commands")
	}
	if e := g.Reconcile(context.Background(), true); e != nil {
		t.Fatal("explicit retry failed", e)
	}
}
func TestP19FirewallReloadMissingJumpWrongOrderAndConflict(t *testing.T) {
	g, m, _ := gateFixture(t)
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	delete(m.chains["ip6tables"], gateChain)
	m.chains["ip6tables"]["PREROUTING"] = [][]string{{"-A", "PREROUTING", "-j", "_NDM_DNS_REDIRECT"}}
	delete(m.sets, gateSet6)
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e != nil {
		t.Fatal("partial IPv6 reload not repaired", e)
	}
	m.mu.Lock()
	m.chains["iptables"]["PREROUTING"] = [][]string{{"-A", "PREROUTING", "-j", "_NDM_DNS_REDIRECT"}}
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e != nil {
		t.Fatal("missing jump not repaired", e)
	}
	m.mu.Lock()
	rs := m.chains["iptables"]["PREROUTING"]
	rs[0], rs[1] = rs[1], rs[0]
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e != nil {
		t.Fatal("stock priority drift not repaired", e)
	}
	m.mu.Lock()
	m.chains["iptables"]["AWGM-DNS"] = [][]string{{"-A", "AWGM-DNS", "-p", "udp", "--dport", "53", "-j", "REDIRECT", "--to-ports", "53"}}
	foreign := []string{"-A", "PREROUTING", "-i", "br0", "-j", "AWGM-DNS"}
	m.chains["iptables"]["PREROUTING"] = append([][]string{foreign}, m.chains["iptables"]["PREROUTING"]...)
	m.mu.Unlock()
	if e := g.Reconcile(context.Background(), false); e == nil || g.Status()["state"] != "CONFLICT" {
		t.Fatal("competing interceptor not reported", e)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	found := false
	for _, r := range m.chains["iptables"]["PREROUTING"] {
		found = found || ruleString(r) == ruleString(foreign)
	}
	if !found || len(m.chains["iptables"]["AWGM-DNS"]) != 1 {
		t.Fatal("foreign firewall changed")
	}
}
func TestP19GateNeverActivatesWhenReadinessOrIPv6Fails(t *testing.T) {
	for _, which := range []string{"listener", "ipv6", "partial"} {
		t.Run(which, func(t *testing.T) {
			g, m, _ := gateFixture(t)
			if which == "listener" {
				g.probe = func(context.Context, string, GatePlan) error { return errors.New("no DNS reply") }
			}
			if which == "ipv6" {
				g.locate = func(name string) (string, error) {
					if name == "ip6tables" {
						return "", errors.New("missing")
					}
					return "/mock", nil
				}
			}
			if which == "partial" {
				m.fail = "ipset -exist add " + gateSet6
			}
			if e := g.Enable(context.Background(), "192.168.10.1:55358"); e == nil {
				t.Fatal("partial unsupported path accepted")
			}
			if g.Status()["active"] != false {
				t.Fatal("failed activation ACTIVE")
			}
			if strings.HasPrefix(m.packet("inet", "br0", "192.168.10.5", "9.9.9.9", "udp", 53), "ROUTERFORGE") {
				t.Fatal("partial IPv4 lease left active")
			}
		})
	}
}
func TestP19DisablePreservesForeignRulesAndPermissionErrors(t *testing.T) {
	g, m, _ := gateFixture(t)
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	foreign := []string{"-A", gateChain, "-j", "ACCEPT"}
	m.chains["iptables"][gateChain] = append(m.chains["iptables"][gateChain], foreign)
	m.mu.Unlock()
	if e := g.Disable(context.Background()); e == nil {
		t.Fatal("foreign rule silently removed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	found := false
	for _, r := range m.chains["iptables"][gateChain] {
		found = found || ruleString(r) == ruleString(foreign)
	}
	if !found {
		t.Fatal("foreign object overwritten")
	}
}
func TestP19StatusDoesNotBlockOnFirewallIO(t *testing.T) {
	g, _, k := gateFixture(t)
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	original := k.Tables.Exec
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	k.Tables.Exec = func(ctx context.Context, b string, a ...string) (string, error) {
		once.Do(func() { close(entered) })
		select {
		case <-release:
			return original(ctx, b, a...)
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	done := make(chan struct{})
	go func() { defer close(done); _ = g.Reconcile(context.Background(), false) }()
	<-entered
	status := make(chan struct{})
	go func() { g.Status(); close(status) }()
	select {
	case <-status:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("status blocked behind kernel commands")
	}
	close(release)
	<-done
	k.Tables.Exec = original
}
func TestP19GatePortAndNoImplicitActivation(t *testing.T) {
	for _, bad := range []string{"0.0.0.0:53", "192.168.10.1:53", "localhost:55358", "192.168.10.1:abc"} {
		if _, e := gatePort(bad); e == nil {
			t.Fatal("unsafe Gate port", bad)
		}
	}
	g, _, _ := gateFixture(t)
	if g.Status()["state"] != "STOCK" || g.Status()["active"] != false {
		t.Fatal("implicit activation")
	}
}

func TestP19MangleInterceptionCannotBypassControllerPreflight(t *testing.T) {
	g, m, _ := gateFixture(t)
	m.mu.Lock()
	m.mangle["iptables"]["PREROUTING"] = [][]string{{"-A", "PREROUTING", "-i", "br0", "-j", "AWGM-TPROXY"}}
	m.mangle["iptables"]["AWGM-TPROXY"] = [][]string{{"-A", "AWGM-TPROXY", "-p", "udp", "--dport", "53", "-j", "TPROXY", "--on-port", "1601"}}
	m.mu.Unlock()
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e == nil {
		t.Fatal("mangle TPROXY conflict ignored")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.mutations) != 0 {
		t.Fatal("preflight conflict mutated kernel")
	}
}
func TestP19CutoverEvictsCachedStockAndOwnedNATMappings(t *testing.T) {
	g, m, k := gateFixture(t)
	ct := k.Conntrack.Table.(*modelCT)
	old := testFlow(t, "192.168.10.55", "9.9.9.9", 17, 53, 1)
	ct.flows = append(ct.flows, old)
	if e := g.Enable(context.Background(), "192.168.10.1:55358"); e != nil {
		t.Fatal(e)
	}
	if len(ct.flows) != 0 || len(ct.deleted) != 1 {
		t.Fatal("stock conntrack mapping would bypass new NAT rules")
	}
	if got := m.packet("inet", "br0", "192.168.10.55", "9.9.9.9", "udp", 53); got != "ROUTERFORGE:55358" {
		t.Fatal(got)
	}
	redirected := testFlow(t, "192.168.10.55", "9.9.9.9", 17, 55358, 2)
	ct.flows = append(ct.flows, redirected)
	m.mu.Lock()
	m.now = m.now.Add(61 * time.Second)
	m.mu.Unlock()
	if got := m.packet("inet", "br0", "192.168.10.55", "9.9.9.9", "udp", 53); got != "STOCK" {
		t.Fatal("expired lease still matches new flow", got)
	}
	if len(ct.flows) != 1 {
		t.Fatal("simulation incorrectly assumes ipset expiry removes conntrack")
	}
	if e := g.Disable(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(ct.flows) != 0 || len(ct.deleted) != 2 {
		t.Fatal("old redirect survived controller disable")
	}
}

func TestP19OwnedInputAndChainOrderRepairPreservesForeignRules(t *testing.T) {
	g, m, _ := gateFixture(t)
	if err := g.Enable(context.Background(), "192.168.10.1:55358"); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	foreign := []string{"-A", "INPUT", "-p", "tcp", "--dport", "22", "-j", "ACCEPT"}
	for _, binary := range []string{"iptables", "ip6tables"} {
		inputs := m.filters[binary]["INPUT"]
		m.filters[binary]["INPUT"] = append([][]string{foreign}, append(inputs, inputs[0])...)
		body := m.chains[binary][gateChain]
		m.chains[binary][gateChain] = append([][]string{body[len(body)-1]}, body...)
	}
	m.mu.Unlock()
	if err := g.Reconcile(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	for _, family := range []string{"inet", "inet6"} {
		src, dst := "192.168.10.5", "9.9.9.9"
		if family == "inet6" {
			src, dst = "fd12:3456::5", "2606:4700:4700::1111"
		}
		if got := m.packet(family, "br0", src, dst, "tcp", 53); got != "ROUTERFORGE:55358" {
			t.Fatal("INPUT blocked redirected DNS", got)
		}
	}
	if strings.HasPrefix(m.packet("inet", "br0", "192.168.10.5", "192.168.10.255", "udp", 53), "ROUTERFORGE") {
		t.Fatal("directed broadcast captured")
	}
	if err := g.Disable(context.Background()); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, binary := range []string{"iptables", "ip6tables"} {
		rs := m.filters[binary]["INPUT"]
		if len(rs) != 1 || ruleString(rs[0]) != ruleString(foreign) {
			t.Fatal("foreign INPUT rule modified", rs)
		}
	}
}
func TestP19RestoreDiscoveryFailureRetainsDesiredAndRecovers(t *testing.T) {
	g, _, _ := gateFixture(t)
	original := g.candidates
	g.candidates = func() (string, []string, error) { return "", nil, errors.New("bridge temporarily unavailable") }
	if err := g.Restore(context.Background(), "192.168.10.1:55358"); err == nil {
		t.Fatal("missing bridge accepted")
	}
	if s := g.Status(); s["desired"] != true || s["state"] != "DEGRADED" {
		t.Fatal(s)
	}
	g.candidates = original
	if err := g.Reconcile(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if g.Status()["active"] != true {
		t.Fatal("saved ON not recoverable")
	}
}
