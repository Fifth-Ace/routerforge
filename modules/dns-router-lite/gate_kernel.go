package lite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const gateOwnerTag = "RF-DNS-LITE:P19"
const controllerLease = 60

type GateFamily struct {
	Binary   string   `json:"binary"`
	Set      string   `json:"set"`
	Family   string   `json:"family"`
	Prefixes []string `json:"prefixes"`
}
type GatePlan struct {
	Version         int          `json:"version"`
	Interface       string       `json:"interface"`
	RouterAddresses []string     `json:"router_addresses"`
	Port            string       `json:"port"`
	Families        []GateFamily `json:"families"`
}
type GateKernel interface {
	Ensure(context.Context, GatePlan) (time.Time, error)
	Remove(context.Context, GatePlan, bool) error
}
type LinuxGate struct {
	Tables        *XTables
	OwnershipPath string
	Conntrack     *ConntrackCleaner
	reorders      []time.Time
}
type GateConflict struct{ Evidence string }

func (e *GateConflict) Error() string { return "DNS interception CONFLICT: " + e.Evidence }
func conflict(format string, a ...any) error {
	return &GateConflict{Evidence: fmt.Sprintf(format, a...)}
}

func words(s string) []string {
	out := strings.Fields(s)
	clean := make([]string, 0, len(out))
	for i := 0; i < len(out); i++ {
		if out[i] == "-m" && i+1 < len(out) && (out[i+1] == "udp" || out[i+1] == "tcp") {
			i++
			continue
		}
		clean = append(clean, strings.Trim(out[i], "\""))
	}
	return clean
}
func ruleString(a []string) string { return strings.Join(a, " ") }
func jumpRule(p GatePlan, legacy bool) []string {
	r := []string{"-A", "PREROUTING", "-i", p.Interface}
	if !legacy {
		r = append(r, "-m", "comment", "--comment", gateOwnerTag)
	}
	return append(r, "-j", gateChain)
}
func chainRules(p GatePlan, f GateFamily, legacy bool) [][]string {
	rules := [][]string{}
	if !legacy {
		multicast := "224.0.0.0/4"
		if f.Family == "inet6" {
			multicast = "ff00::/8"
		}
		rules = append(rules, []string{"-A", gateChain, "-d", multicast, "-m", "comment", "--comment", gateOwnerTag, "-j", "RETURN"})
		if f.Family == "inet" {
			rules = append(rules, []string{"-A", gateChain, "-d", "255.255.255.255/32", "-m", "comment", "--comment", gateOwnerTag, "-j", "RETURN"})
		}
		if f.Family == "inet" {
			for _, prefix := range f.Prefixes {
				if broadcast := prefixBroadcast(prefix); broadcast != nil {
					rules = append(rules, []string{"-A", gateChain, "-d", broadcast.String() + "/32", "-m", "comment", "--comment", gateOwnerTag, "-j", "RETURN"})
				}
			}
		}
	}
	for _, proto := range []string{"udp", "tcp"} {
		r := []string{"-A", gateChain, "-p", proto, "--dport", "53", "-m", "set", "--match-set", f.Set, "src"}
		if !legacy {
			r = append(r, "-m", "comment", "--comment", gateOwnerTag)
		}
		rules = append(rules, append(r, "-j", "REDIRECT", "--to-ports", p.Port))
	}
	return rules
}

type familyView struct {
	Inputs  map[string]int
	InputOK bool
	Ordered []string
	Counts  map[string]int
	Chain   bool
	Rules   map[string]bool
	Jumps   int
	First   bool
	Set     bool
	Lease   map[string]int
	ReadAt  time.Time
}

func argValue(a []string, key string) string {
	for i := 0; i+1 < len(a); i++ {
		if a[i] == key {
			return a[i+1]
		}
	}
	return ""
}
func includesDNS(port string) bool {
	if port == "" {
		return true
	}
	for _, part := range strings.Split(port, ",") {
		rangeParts := strings.Split(part, ":")
		n, e := strconv.Atoi(rangeParts[0])
		if e != nil {
			return true
		}
		if len(rangeParts) == 1 && n == 53 {
			return true
		}
		if len(rangeParts) == 2 {
			end, e := strconv.Atoi(rangeParts[1])
			if e != nil || n <= 53 && end >= 53 {
				return true
			}
		}
	}
	return false
}
func mayCaptureDNS(chain string, rules map[string][][]string, seen map[string]bool) bool {
	if seen[chain] {
		return true
	}
	seen[chain] = true
	for _, r := range rules[chain] {
		proto := argValue(r, "-p")
		if proto != "" && proto != "udp" && proto != "tcp" {
			continue
		}
		port := argValue(r, "--dport")
		if port == "" {
			port = argValue(r, "--dports")
		}
		if !includesDNS(port) {
			continue
		}
		target := argValue(r, "-j")
		if target == "REDIRECT" || target == "DNAT" || target == "TPROXY" {
			return true
		}
		if target != "" && target != "RETURN" && target != "ACCEPT" && target != "DROP" && target != "REJECT" {
			if strings.HasPrefix(target, "AWGM-") || mayCaptureDNS(target, rules, seen) {
				return true
			}
		}
	}
	return false
}
func (k *LinuxGate) view(ctx context.Context, p GatePlan, f GateFamily, legacy bool, ignoreCompetition ...bool) (familyView, error) {
	v := familyView{Rules: map[string]bool{}, Counts: map[string]int{}, Lease: map[string]int{}}
	var inputErr error
	v.Inputs, v.InputOK, inputErr = k.inputView(ctx, p, f)
	if inputErr != nil {
		return v, inputErr
	}
	if len(ignoreCompetition) == 0 || !ignoreCompetition[0] {
		if e := k.auditMangle(ctx, p, f); e != nil {
			return v, e
		}
	}
	dump, err := k.Tables.Run(ctx, f.Binary, "-t", "nat", "-S")
	if err != nil {
		return v, err
	}
	all := map[string][][]string{}
	expected := map[string]bool{}
	for _, r := range chainRules(p, f, legacy) {
		expected[ruleString(r)] = true
	}
	for _, line := range strings.Split(dump, "\n") {
		a := words(line)
		if len(a) < 2 {
			continue
		}
		if a[0] == "-N" && a[1] == gateChain {
			v.Chain = true
		}
		if a[0] != "-A" {
			continue
		}
		all[a[1]] = append(all[a[1]], a)
		if argValue(a, "--match-set") == f.Set && !expected[ruleString(a)] {
			return v, conflict("foreign NAT reference to owned lease set: %s", line)
		}
		if a[1] == gateChain {
			v.Chain = true
			if !expected[ruleString(a)] {
				return v, conflict("foreign rule in %s/%s: %s", f.Binary, gateChain, line)
			}
			v.Rules[ruleString(a)] = true
			v.Ordered = append(v.Ordered, ruleString(a))
			v.Counts[ruleString(a)]++
		}
		target := argValue(a, "-j")
		if target == "" {
			target = argValue(a, "-g")
		}
		if target == gateChain && ruleString(a) != ruleString(jumpRule(p, legacy)) {
			return v, conflict("unowned reference to %s: %s", gateChain, line)
		}
	}
	for index, r := range all["PREROUTING"] {
		if ruleString(r) == ruleString(jumpRule(p, legacy)) {
			v.Jumps++
			v.First = index == 0
			continue
		}
		if len(ignoreCompetition) > 0 && ignoreCompetition[0] {
			continue
		}
		iface := argValue(r, "-i")
		if iface != "" && iface != p.Interface {
			continue
		}
		proto := argValue(r, "-p")
		if proto != "" && proto != "udp" && proto != "tcp" {
			continue
		}
		port := argValue(r, "--dport")
		if port == "" {
			port = argValue(r, "--dports")
		}
		if !includesDNS(port) {
			continue
		}
		target := argValue(r, "-j")
		if strings.HasPrefix(target, "_NDM_") {
			continue
		}
		if target == "REDIRECT" || target == "DNAT" || strings.HasPrefix(target, "AWGM-") || mayCaptureDNS(target, all, map[string]bool{}) {
			return v, conflict("competing DNS interception: %s", ruleString(r))
		}
	}
	set, e := k.readLeaseSet(ctx, f)
	if e != nil {
		return v, e
	}
	v.Set = set.Set
	v.Lease = set.Lease
	v.ReadAt = set.ReadAt
	return v, nil
}
func (k *LinuxGate) readLeaseSet(ctx context.Context, f GateFamily) (familyView, error) {
	v := familyView{Lease: map[string]int{}}
	v.ReadAt = time.Now()
	saved, err := k.Tables.Run(ctx, "ipset", "save", f.Set)
	if err != nil {
		if gateMissingObject(err, saved) {
			return v, nil
		}
		return v, err
	}
	for _, line := range strings.Split(saved, "\n") {
		a := words(line)
		if len(a) == 0 {
			continue
		}
		if len(a) < 3 || a[1] != f.Set {
			return v, conflict("unexpected ipset output for %s", f.Set)
		}
		if a[0] == "create" {
			if a[2] != "hash:net" || argValue(a, "family") != f.Family || argValue(a, "timeout") == "" || argValue(a, "timeout") == "0" {
				return v, conflict("unsupported lease set %s", line)
			}
			v.Set = true
		} else if a[0] == "add" {
			_, network, e := net.ParseCIDR(a[2])
			if e != nil {
				return v, conflict("invalid set member %s", line)
			}
			prefix := network.String()
			found := false
			for _, want := range f.Prefixes {
				found = found || prefix == want
			}
			if !found {
				return v, conflict("foreign prefix in %s: %s", f.Set, prefix)
			}
			ttl, e := strconv.Atoi(argValue(a, "timeout"))
			if e != nil || ttl <= 0 {
				return v, conflict("non-expiring or invalid lease: %s", line)
			}
			v.Lease[prefix] = ttl
		} else {
			return v, conflict("unrecognized ipset statement %s", line)
		}
	}
	return v, nil
}
func (k *LinuxGate) receipt() (GatePlan, error) {
	var p GatePlan
	if k.OwnershipPath == "" {
		return p, errors.New("Gate ownership path unavailable")
	}
	e := readState(k.OwnershipPath, &p, 8192)
	if e == nil {
		e = validateGatePlan(p)
	}
	return p, e
}
func (k *LinuxGate) writeReceipt(p GatePlan) error {
	if e := validateGatePlan(p); e != nil {
		return e
	}
	b, e := jsonMarshal(p)
	if e != nil {
		return e
	}
	return atomicFile(k.OwnershipPath, b)
}
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

func (k *LinuxGate) Ensure(ctx context.Context, p GatePlan) (until time.Time, err error) {
	if e := validateGatePlan(p); e != nil {
		return time.Time{}, e
	}
	if e := k.Conntrack.CheckPlan(ctx, p); e != nil {
		return time.Time{}, fmt.Errorf("conntrack preflight: %w", e)
	}
	opened := false
	defer func() {
		if err != nil && opened {
			cleanup, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if e := k.Deactivate(cleanup, p); e != nil {
				err = fmt.Errorf("%v; deactivation pending: %w", err, e)
			}
		}
	}()
	if old, err := k.receipt(); err == nil && !reflect.DeepEqual(old, p) {
		if err = k.Remove(ctx, old, false); err != nil {
			return time.Time{}, err
		}
	} else if err != nil && !os.IsNotExist(err) {
		return time.Time{}, err
	}
	views := make([]familyView, len(p.Families))
	repair := false
	for i, f := range p.Families {
		v, e := k.view(ctx, p, f, false)
		if e != nil {
			return time.Time{}, e
		}
		views[i] = v
		if v.Jumps > 0 && (!v.First || !v.InputOK) {
			now := time.Now()
			recent := k.reorders[:0]
			for _, at := range k.reorders {
				if now.Sub(at) < time.Minute {
					recent = append(recent, at)
				}
			}
			k.reorders = recent
			if len(k.reorders) >= 2 {
				return time.Time{}, conflict("repeated priority drift; refusing a firewall priority fight")
			}
			k.reorders = append(k.reorders, now)
		}
		repair = repair || !v.InputOK || v.Jumps != 1 || !v.First || !v.Chain || !reflect.DeepEqual(v.Ordered, orderedRules(p, f))
	}
	if err := k.writeReceipt(p); err != nil {
		return time.Time{}, err
	}
	if repair {
		// Empty leases before any structural change. No lasting half-enabled
		// family is accepted if a later operation fails.
		for i, f := range p.Families {
			if views[i].Set {
				if _, e := k.Tables.Run(ctx, "ipset", "flush", f.Set); e != nil {
					return time.Time{}, e
				}
			}
		}
	}
	for i, f := range p.Families {
		v := views[i]
		if !v.Set {
			if _, e := k.Tables.Run(ctx, "ipset", "create", f.Set, "hash:net", "family", f.Family, "timeout", strconv.Itoa(controllerLease)); e != nil {
				return time.Time{}, e
			}
		}
		if !v.InputOK {
			if e := k.repairInputs(ctx, p, f, v.Inputs); e != nil {
				return time.Time{}, e
			}
		}
		if !v.Chain {
			if _, e := k.Tables.Run(ctx, f.Binary, "-t", "nat", "-N", gateChain); e != nil {
				return time.Time{}, e
			}
		}
		if v.Chain && !reflect.DeepEqual(v.Ordered, orderedRules(p, f)) {
			if _, e := k.Tables.Run(ctx, f.Binary, "-t", "nat", "-F", gateChain); e != nil {
				return time.Time{}, e
			}
			v.Rules = map[string]bool{}
		}
		for _, r := range chainRules(p, f, false) {
			if !v.Rules[ruleString(r)] {
				if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "nat"}, r...)...); e != nil {
					return time.Time{}, e
				}
			}
		}
		if v.Jumps != 1 || !v.First {
			for n := 0; n < v.Jumps; n++ {
				r := jumpRule(p, false)
				r[0] = "-D"
				if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "nat"}, r...)...); e != nil {
					return time.Time{}, e
				}
			}
			r := jumpRule(p, false)
			r = append([]string{"-I", "PREROUTING", "1"}, r[2:]...)
			if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "nat"}, r...)...); e != nil {
				return time.Time{}, e
			}
		}
	}
	// Re-read both families and ordering before opening either lease set.
	for _, f := range p.Families {
		v, e := k.view(ctx, p, f, false)
		if e != nil {
			return time.Time{}, e
		}
		if !v.InputOK || v.Jumps != 1 || !v.First || !reflect.DeepEqual(v.Ordered, orderedRules(p, f)) {
			return time.Time{}, conflict("topology changed while enabling %s", f.Binary)
		}
	}
	opened = true
	for _, f := range p.Families {
		for _, prefix := range f.Prefixes {
			if _, e := k.Tables.Run(ctx, "ipset", "-exist", "add", f.Set, prefix, "timeout", strconv.Itoa(controllerLease)); e != nil {
				for _, cleanup := range p.Families {
					_, _ = k.Tables.Run(ctx, "ipset", "flush", cleanup.Set)
				}
				return time.Time{}, e
			}
		}
	}
	if k.Conntrack == nil {
		return time.Time{}, errors.New("safe conntrack cleanup unavailable")
	}
	if e := k.Conntrack.Evict(ctx, p, false); e != nil {
		return time.Time{}, e
	}
	until = time.Now().Add(controllerLease * time.Second)
	for _, f := range p.Families {
		v, e := k.view(ctx, p, f, false)
		if e != nil {
			return time.Time{}, e
		}
		if !v.InputOK || v.Jumps != 1 || !v.First || !reflect.DeepEqual(v.Ordered, orderedRules(p, f)) {
			return time.Time{}, conflict("jump or owned rule order changed after activation")
		}
		for _, prefix := range f.Prefixes {
			ttl := v.Lease[prefix]
			if ttl < 10 {
				return time.Time{}, errors.New("DNS lease missing or too close to expiry")
			}
			expires := v.ReadAt.Add(time.Duration(ttl-1) * time.Second)
			if expires.Before(until) {
				until = expires
			}
		}
	}
	return until, nil
}
func (k *LinuxGate) Remove(ctx context.Context, p GatePlan, legacy bool) error {
	if err := validateGatePlan(p); err != nil {
		return err
	}
	views := make([]familyView, len(p.Families))
	for i, f := range p.Families {
		v, e := k.view(ctx, p, f, legacy, true)
		if e != nil {
			return e
		}
		views[i] = v
	}
	for i, f := range p.Families {
		if views[i].Set {
			if _, e := k.Tables.Run(ctx, "ipset", "flush", f.Set); e != nil {
				return e
			}
		}
	}
	for i, f := range p.Families {
		v := views[i]
		for n := 0; n < v.Jumps; n++ {
			r := jumpRule(p, legacy)
			r[0] = "-D"
			if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "nat"}, r...)...); e != nil {
				return e
			}
		}
		for _, r := range chainRules(p, f, legacy) {
			for n := 0; n < v.Counts[ruleString(r)]; n++ {
				argv := append([]string{}, r...)
				argv[0] = "-D"
				if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "nat"}, argv...)...); e != nil {
					return e
				}
			}
		}
		if v.Chain {
			if _, e := k.Tables.Run(ctx, f.Binary, "-t", "nat", "-X", gateChain); e != nil {
				return e
			}
		}
		for _, r := range inputRules(p, f) {
			for n := 0; n < v.Inputs[ruleString(r)]; n++ {
				argv := append([]string{}, r...)
				argv[0] = "-D"
				if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "filter"}, argv...)...); e != nil {
					return e
				}
			}
		}
		if v.Set {
			if _, e := k.Tables.Run(ctx, "ipset", "destroy", f.Set); e != nil {
				return e
			}
		}
	}
	if k.Conntrack == nil {
		return errors.New("safe conntrack cleanup unavailable")
	}
	var ctErr error
	if legacy {
		ctErr = k.Conntrack.evictLegacy(ctx, p)
	} else {
		ctErr = k.Conntrack.Evict(ctx, p, true)
	}
	if ctErr != nil {
		return ctErr
	}
	if k.OwnershipPath != "" {
		if e := os.Remove(k.OwnershipPath); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}

func validateGatePlan(p GatePlan) error {
	if p.Version != 1 || p.Interface != gateInterface || len(p.Families) < 1 || len(p.Families) > 2 {
		return errors.New("invalid Gate ownership plan")
	}
	if _, e := gatePort(net.JoinHostPort("192.168.10.1", p.Port)); e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, f := range p.Families {
		if seen[f.Binary] || len(f.Prefixes) == 0 || len(f.Prefixes) > 4 {
			return errors.New("invalid Gate family ownership")
		}
		seen[f.Binary] = true
		if !(f.Binary == "iptables" && f.Set == gateSet4 && f.Family == "inet" || f.Binary == "ip6tables" && f.Set == gateSet6 && f.Family == "inet6") {
			return errors.New("unrecognized Gate executable, set or family")
		}
		prefixes := map[string]bool{}
		for _, raw := range f.Prefixes {
			ip, n, e := net.ParseCIDR(raw)
			if e != nil || n.String() != raw || prefixes[raw] || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback() {
				return errors.New("invalid owned Gate prefix")
			}
			ones, bits := n.Mask.Size()
			if f.Family == "inet" && (bits != 32 || ones < 8 || !ip.IsPrivate()) || f.Family == "inet6" && (bits != 128 || ones < 32) {
				return errors.New("unsafe owned Gate prefix")
			}
			prefixes[raw] = true
		}
	}
	if len(p.RouterAddresses) < 1 || len(p.RouterAddresses) > 8 {
		return errors.New("Gate router address ownership missing")
	}
	addresses := map[string]bool{}
	for _, raw := range p.RouterAddresses {
		ip := net.ParseIP(raw)
		if ip == nil || ip.String() != raw || addresses[raw] || ip.IsLoopback() || ip.IsMulticast() {
			return errors.New("invalid owned router address")
		}
		addresses[raw] = true
		inside := false
		for _, f := range p.Families {
			for _, prefix := range f.Prefixes {
				_, n, _ := net.ParseCIDR(prefix)
				inside = inside || n.Contains(ip)
			}
		}
		if !inside {
			return errors.New("router address outside managed prefixes")
		}
	}

	return nil
}
func (k *LinuxGate) Audit(ctx context.Context, p GatePlan) error {
	if old, e := k.receipt(); e == nil {
		p = old
	} else if !os.IsNotExist(e) {
		return e
	}
	if e := validateGatePlan(p); e != nil {
		return e
	}
	if k.Conntrack == nil {
		return errors.New("safe conntrack cleanup unavailable")
	}
	if err := k.Conntrack.CheckPlan(ctx, p); err != nil {
		return fmt.Errorf("conntrack preflight: %w", err)
	}
	for _, f := range p.Families {
		if _, e := k.view(ctx, p, f, false); e != nil {
			return e
		}
	}
	return nil
}

func (k *LinuxGate) auditMangle(ctx context.Context, p GatePlan, f GateFamily) error {
	dump, err := k.Tables.Run(ctx, f.Binary, "-t", "mangle", "-S")
	if err != nil {
		return fmt.Errorf("mangle topology unavailable: %w", err)
	}
	return inspectMangleDNS(dump, p, f)
}

// Deactivate does not require the xtables lock. A dead listener must not keep
// receiving cached NAT flows while NDM holds that lock. Both lease ownership
// and exact conntrack identities are still checked before any mutation.
func (k *LinuxGate) Deactivate(ctx context.Context, p GatePlan) error {
	if e := validateGatePlan(p); e != nil {
		return e
	}
	views := make([]familyView, len(p.Families))
	for i, f := range p.Families {
		v, e := k.readLeaseSet(ctx, f)
		if e != nil {
			return e
		}
		views[i] = v
	}
	for i, f := range p.Families {
		if views[i].Set {
			if _, e := k.Tables.Run(ctx, "ipset", "flush", f.Set); e != nil {
				return e
			}
		}
	}
	if k.Conntrack == nil {
		return errors.New("safe conntrack cleanup unavailable")
	}
	return k.Conntrack.Evict(ctx, p, true)
}

func orderedRules(p GatePlan, f GateFamily) []string {
	out := []string{}
	for _, r := range chainRules(p, f, false) {
		out = append(out, ruleString(r))
	}
	return out
}

func prefixBroadcast(raw string) net.IP {
	_, network, e := net.ParseCIDR(raw)
	if e != nil || network.IP.To4() == nil {
		return nil
	}
	ip := append(net.IP{}, network.IP.To4()...)
	for i := range ip {
		ip[i] |= ^network.Mask[i]
	}
	return ip
}

func inputRules(p GatePlan, f GateFamily) [][]string {
	out := [][]string{}
	for _, proto := range []string{"udp", "tcp"} {
		out = append(out, []string{"-A", "INPUT", "-i", p.Interface, "-p", proto, "--dport", p.Port, "-m", "set", "--match-set", f.Set, "src", "-m", "comment", "--comment", gateOwnerTag, "-j", "ACCEPT"})
	}
	return out
}
func (k *LinuxGate) inputView(ctx context.Context, p GatePlan, f GateFamily) (map[string]int, bool, error) {
	dump, e := k.Tables.Run(ctx, f.Binary, "-t", "filter", "-S")
	if e != nil {
		return nil, false, e
	}
	expected := map[string]bool{}
	for _, r := range inputRules(p, f) {
		expected[ruleString(r)] = true
	}
	counts := map[string]int{}
	position := 0
	front := true
	for _, line := range strings.Split(dump, "\n") {
		r := words(line)
		if len(r) < 2 || r[0] != "-A" {
			continue
		}
		key := ruleString(r)
		if argValue(r, "--match-set") == f.Set && !expected[key] {
			return nil, false, conflict("foreign filter reference to owned lease set: %s", line)
		}
		if argValue(r, "--comment") == gateOwnerTag {
			if !expected[key] {
				return nil, false, conflict("unrecognized owned INPUT rule: %s", line)
			}
			counts[key]++
			front = front && position < 2
		}
		if r[1] == "INPUT" {
			position++
		}
	}
	for key := range expected {
		front = front && counts[key] == 1
	}
	return counts, front, nil
}
func (k *LinuxGate) repairInputs(ctx context.Context, p GatePlan, f GateFamily, counts map[string]int) error {
	for _, r := range inputRules(p, f) {
		for n := 0; n < counts[ruleString(r)]; n++ {
			argv := append([]string{}, r...)
			argv[0] = "-D"
			if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "filter"}, argv...)...); e != nil {
				return e
			}
		}
	}
	rules := inputRules(p, f)
	for i := len(rules) - 1; i >= 0; i-- {
		r := append([]string{"-I", "INPUT", "1"}, rules[i][2:]...)
		if _, e := k.Tables.Run(ctx, f.Binary, append([]string{"-t", "filter"}, r...)...); e != nil {
			return e
		}
	}
	return nil
}
