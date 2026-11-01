package lite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
)

type legacyInspection struct {
	objects map[string]bool
	nat     map[string]string
	flows   []CTFlow
}

func hasLegacyName(line string, set string) bool {
	for _, s := range words(line) {
		if s == gateChain || s == set || s == gateOwnerTag {
			return true
		}
	}
	return false
}

// All reads, no activation preflight: even absent jumps can leave chains, sets,
// foreign references, or cached redirect tuples. Failure to inspect is an error.
func (k *LinuxGate) inspectLegacy(ctx context.Context) (legacyInspection, error) {
	s := legacyInspection{objects: map[string]bool{}, nat: map[string]string{}}
	for _, f := range []GateFamily{{Binary: "iptables", Set: gateSet4}, {Binary: "ip6tables", Set: gateSet6}} {
		for _, table := range []string{"nat", "filter", "mangle"} {
			dump, err := k.Tables.Run(ctx, f.Binary, "-t", table, "-S")
			if err != nil {
				return s, fmt.Errorf("legacy %s/%s inspection: %w", f.Binary, table, err)
			}
			if table == "nat" {
				s.nat[f.Binary] = dump
			}
			builtin := "PREROUTING"
			if table == "filter" {
				builtin = "INPUT"
			}
			proved := false
			for _, line := range strings.Split(dump, "\n") {
				r := words(line)
				proved = proved || (len(r) == 3 && r[0] == "-P" && r[1] == builtin)
			}
			if !proved {
				return s, fmt.Errorf("legacy %s/%s dump is incomplete", f.Binary, table)
			}
			for _, line := range strings.Split(dump, "\n") {
				if hasLegacyName(line, f.Set) {
					if table != "nat" {
						return s, conflict("legacy ownership cannot include %s reference: %s", table, line)
					}
					s.objects[f.Binary] = true
				}
			}
		}
		dump, err := k.Tables.Run(ctx, "ipset", "save", f.Set)
		if err != nil {
			if !gateMissingObject(err, dump) {
				return s, err
			}
		} else {
			if strings.TrimSpace(dump) == "" {
				return s, errors.New("empty successful legacy ipset dump is unproven")
			}
			s.objects[f.Binary] = true
		}
	}
	if k.Conntrack == nil || k.Conntrack.Table == nil {
		return s, errors.New("legacy conntrack inspection unavailable")
	}
	var err error
	s.flows, err = k.Conntrack.Table.Dump(ctx)
	return s, err
}
func dnsRedirectPort(f CTFlow, port uint16) bool {
	return f.Original.DstPort == 53 && (f.Original.Proto == 6 || f.Original.Proto == 17) && f.Reply.SrcPort == port
}
func routerReply(p GatePlan, f CTFlow) bool {
	for _, raw := range p.RouterAddresses {
		if net.ParseIP(raw).Equal(f.Reply.Src) {
			return true
		}
	}
	return false
}
func (g *GateManager) legacyPlan(listen string) (GatePlan, error) {
	if g.topology == nil {
		return g.makePlan(listen)
	}
	t, err := g.topology()
	if err != nil {
		return GatePlan{}, err
	}
	v4, v6, _ := t.legacyCandidates()
	return planFromCandidates(listen, v4, v6, true)
}

// StageLegacyCleanup only builds a cleanup scope if read-only inspection finds
// residual objects/tuples. A clean STOCK gate does not need new Gate readiness,
// topology validation, ipset creation, consent or listener IPv6 activation.
func (g *GateManager) StageLegacyCleanup(ctx context.Context, path, listen string) error {
	portRaw, err := gatePort(listen)
	if err != nil {
		return err
	}
	port, _ := strconv.Atoi(portRaw)
	host, _, _ := net.SplitHostPort(listen)
	k, ok := g.kernel.(*LinuxGate)
	if !ok {
		return errors.New("legacy cleanup backend unavailable")
	}
	var staged GatePlan
	stagedErr := readState(path+".legacy-gate-cleanup.json", &staged, 8192)
	if stagedErr == nil {
		if err = validateGatePlan(staged); err != nil {
			return err
		}
	} else if !os.IsNotExist(stagedErr) {
		return stagedErr
	}
	s, err := k.inspectLegacy(ctx)
	if err != nil {
		return err
	}
	need := len(s.objects) > 0
	for _, f := range s.flows {
		if dnsRedirectPort(f, uint16(port)) && !f.Original.Src.Equal(net.ParseIP(host)) && (f.Family == 10 || f.Reply.Src.Equal(net.ParseIP(host))) {
			need = true
		}
	}
	if !need {
		return nil
	}
	p, err := g.legacyPlan(listen)
	if err != nil {
		return err
	}
	tuples := map[string]bool{}
	for _, f := range s.flows {
		if !dnsRedirectPort(f, uint16(port)) || !routerReply(p, f) {
			continue
		}
		routerSource := false
		for _, raw := range p.RouterAddresses {
			routerSource = routerSource || net.ParseIP(raw).Equal(f.Original.Src)
		}
		if routerSource {
			continue
		}
		if !managedFlow(p, f) {
			return conflict("redirect tuple to legacy listener has unproven LAN ownership")
		}
		if f.Original.Src.IsLinkLocalUnicast() || f.Original.Dst.IsLinkLocalUnicast() || f.Reply.Src.IsLinkLocalUnicast() {
			return conflict("legacy IPv6 link-local conntrack scope is unproven; no automatic cleanup")
		}
		if err = validateManagedFlow(p, f); err != nil {
			return err
		}
		binary := "iptables"
		if f.Family == 10 {
			binary = "ip6tables"
		}
		tuples[binary] = true
	}
	selected := []GateFamily{}
	covered := map[string]bool{}
	for _, f := range p.Families {
		if !s.objects[f.Binary] && !tuples[f.Binary] {
			continue
		}
		for _, line := range strings.Split(s.nat[f.Binary], "\n") {
			r := words(line)
			target := argValue(r, "-j")
			if len(r) > 2 && r[1] != gateChain && (target == "REDIRECT" && argValue(r, "--to-ports") == p.Port || target == "DNAT" && strings.HasSuffix(argValue(r, "--to-destination"), ":"+p.Port)) {
				return conflict("foreign redirect to legacy listener: %s", line)
			}
		}
		v, e := k.view(ctx, p, f, true, true)
		if e != nil {
			return e
		}
		if !tuples[f.Binary] && v.Jumps == 0 && len(v.Rules) == 0 {
			return conflict("isolated legacy objects have no proven rule ownership: %s", f.Binary)
		}
		selected = append(selected, f)
		covered[f.Binary] = true
	}
	for family := range s.objects {
		if !covered[family] {
			return conflict("legacy %s resources have no proven current topology scope", family)
		}
	}
	if len(selected) == 0 {
		return nil
	} // Only unrelated high-port IPv6 flows existed.
	p.Families = selected
	addresses := []string{}
	for _, raw := range p.RouterAddresses {
		ip := net.ParseIP(raw)
		for _, f := range selected {
			found := false
			for _, prefix := range f.Prefixes {
				_, n, _ := net.ParseCIDR(prefix)
				found = found || n.Contains(ip)
			}
			if found {
				addresses = append(addresses, raw)
				break
			}
		}
	}
	p.RouterAddresses = addresses
	if err = validateGatePlan(p); err != nil {
		return err
	}
	if stagedErr == nil {
		if !reflect.DeepEqual(staged, p) {
			return errors.New("staged legacy cleanup differs from current ownership evidence")
		}
		return nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return atomicFile(path+".legacy-gate-cleanup.json", b)
}

// PrepareLegacyUpgrade is shared by the CLI and regression tests. It validates
// the exact legacy applied revision before staging any handoff metadata.
func (g *GateManager) PrepareLegacyUpgrade(ctx context.Context, path string, e UpgradeEvidence) (Intent, error) {
	store := IntentStore{ConfigPath: path}
	if existing, err := store.Load(); err == nil {
		checked, err := PrepareUpgrade(path, e)
		if err != nil {
			return checked, err
		}
		if existing.ServiceEnabled {
			c, err := store.Applied(existing)
			if err != nil {
				return existing, err
			}
			if err = g.StageLegacyCleanup(ctx, path, c.Service.Listen); err != nil {
				return existing, err
			}
		}
		return checked, nil
	} else if !os.IsNotExist(err) {
		return existing, err
	}
	if e.Connection.State != "disconnected" {
		return Intent{}, errors.New("restore legacy client profile before upgrade")
	}
	if e.Active {
		c, rev, err := dr.NewConfigStore(path + ".service-active.json").Load()
		if err != nil {
			return Intent{}, err
		}
		if c.Service == nil || rev != e.AppliedRevision {
			return Intent{}, errors.New("live DNS revision does not match the legacy applied snapshot")
		}
		if err = g.StageLegacyCleanup(ctx, path, c.Service.Listen); err != nil {
			return Intent{}, err
		}
	}
	return PrepareUpgrade(path, e)
}
