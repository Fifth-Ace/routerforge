package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	flowSocketSnapshotMax  = 4096
	flowOwnerPIDBudget     = 2048
	flowOwnerFDBudget      = 16384
	flowPolicyLookupBudget = 32
)

type flowSocketAttribution struct {
	Available bool   `json:"available"`
	Source    string `json:"source"`
	PID       int    `json:"pid,omitempty"`
	Process   string `json:"process,omitempty"`
	Inode     string `json:"inode,omitempty"`
	Match     string `json:"match,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type flowPolicyCandidate struct {
	Priority int    `json:"priority"`
	Table    string `json:"table,omitempty"`
	From     string `json:"from,omitempty"`
	To       string `json:"to,omitempty"`
	Mark     string `json:"mark,omitempty"`
	IIF      string `json:"iif,omitempty"`
	OIF      string `json:"oif,omitempty"`
	Match    string `json:"match"`
}

type flowPolicyExplanation struct {
	Available      bool                  `json:"available"`
	Source         string                `json:"source"`
	Mark           string                `json:"mark,omitempty"`
	Candidates     []flowPolicyCandidate `json:"candidates"`
	KernelDecision kernelRouteDecision   `json:"kernel_decision"`
	Lookup         bool                  `json:"lookup_performed"`
	Reason         string                `json:"reason,omitempty"`
}

type flowEnrichmentStats struct {
	SocketSource       string `json:"socket_source"`
	SocketRecords      int    `json:"socket_records"`
	OwnerPIDsScanned   int    `json:"owner_pids_scanned"`
	OwnerFDsScanned    int    `json:"owner_fds_scanned"`
	OwnerBudgetHit     bool   `json:"owner_budget_hit"`
	PolicySource       string `json:"policy_source"`
	PolicyRules        int    `json:"policy_rules"`
	RouteLookups       int    `json:"route_lookups"`
	RouteLookupBudget  int    `json:"route_lookup_budget"`
	RouteLookupLimited bool   `json:"route_lookup_limited"`
}

type flowSocketKey struct {
	Protocol   string
	LocalIP    string
	LocalPort  string
	RemoteIP   string
	RemotePort string
}

type flowSocketRecord struct {
	Inode string
	Key   flowSocketKey
}

type flowSocketOwner struct {
	PID     int
	Process string
}

type flowSocketMatch struct {
	Record flowSocketRecord
	Match  string
}

func enrichFlowExplorer(ctx context.Context, entries []flowExplorerEntry) flowEnrichmentStats {
	sockets, socketSource := readFlowSocketSnapshot(flowSocketSnapshotMax)
	matches := make([]flowSocketMatch, len(entries))
	inodes := map[string]bool{}
	for i := range entries {
		match := matchFlowSocket(entries[i], sockets)
		matches[i] = match
		if match.Record.Inode != "" {
			inodes[match.Record.Inode] = true
		}
	}

	owners, pidsScanned, fdsScanned, budgetHit := resolveFlowSocketOwners(
		inodes,
		flowOwnerPIDBudget,
		flowOwnerFDBudget,
	)

	for i := range entries {
		match := matches[i]
		if match.Record.Inode == "" {
			entries[i].Socket = flowSocketAttribution{
				Available: false,
				Source:    socketSource,
				Reason:    "no matching local socket; flow may be forwarded, expired, or outside snapshot",
			}
			continue
		}
		owner, ok := owners[match.Record.Inode]
		entries[i].Socket = flowSocketAttribution{
			Available: ok,
			Source:    socketSource + "+proc-pid-fd",
			PID:       owner.PID,
			Process:   owner.Process,
			Inode:     match.Record.Inode,
			Match:     match.Match,
		}
		if !ok {
			entries[i].Socket.Reason = "socket inode matched but owner was not resolved within bounded process/fd scan"
		}
	}

	rules, policySource := readFlowPolicyRules(ctx)
	cache := map[string]kernelRouteDecision{}
	lookups := 0
	lookupLimited := false

	for i := range entries {
		explanation := explainFlowPolicy(entries[i], rules)
		explanation.Source = policySource
		target := strings.TrimSpace(entries[i].EffectiveDestination)

		if policySource == "ip-rule" && net.ParseIP(target) != nil && net.ParseIP(target).To4() != nil {
			key := target + "|" + normalizeFlowMark(entries[i].Mark)
			if decision, exists := cache[key]; exists {
				explanation.KernelDecision = decision
				explanation.Lookup = true
			} else if lookups < flowPolicyLookupBudget {
				decision := flowKernelRouteGet(ctx, target, entries[i].Mark)
				cache[key] = decision
				lookups++
				explanation.KernelDecision = decision
				explanation.Lookup = true
			} else {
				lookupLimited = true
				explanation.Reason = appendFlowReason(explanation.Reason, "kernel route-get budget exhausted")
			}
		}
		entries[i].Policy = explanation
	}

	return flowEnrichmentStats{
		SocketSource:       socketSource,
		SocketRecords:      len(sockets),
		OwnerPIDsScanned:   pidsScanned,
		OwnerFDsScanned:    fdsScanned,
		OwnerBudgetHit:     budgetHit,
		PolicySource:       policySource,
		PolicyRules:        len(rules),
		RouteLookups:       lookups,
		RouteLookupBudget:  flowPolicyLookupBudget,
		RouteLookupLimited: lookupLimited,
	}
}

func readFlowSocketSnapshot(limit int) (map[flowSocketKey]flowSocketRecord, string) {
	if limit <= 0 || limit > flowSocketSnapshotMax {
		limit = flowSocketSnapshotMax
	}
	out := map[flowSocketKey]flowSocketRecord{}
	sources := []struct {
		Path     string
		Protocol string
	}{
		{"/proc/net/tcp", "TCP"},
		{"/proc/net/tcp6", "TCP"},
		{"/proc/net/udp", "UDP"},
		{"/proc/net/udp6", "UDP"},
	}

	for _, source := range sources {
		if len(out) >= limit {
			break
		}
		file, err := os.Open(source.Path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		first := true
		for scanner.Scan() && len(out) < limit {
			if first {
				first = false
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) < 10 {
				continue
			}
			localIP, localPort, ok := decodeFlowSocketEndpoint(fields[1])
			if !ok {
				continue
			}
			remoteIP, remotePort, ok := decodeFlowSocketEndpoint(fields[2])
			if !ok {
				continue
			}
			key := flowSocketKey{
				Protocol:   source.Protocol,
				LocalIP:    localIP,
				LocalPort:  localPort,
				RemoteIP:   remoteIP,
				RemotePort: remotePort,
			}
			if _, exists := out[key]; exists {
				continue
			}
			out[key] = flowSocketRecord{
				Inode: fields[9],
				Key:   key,
			}
		}
		_ = file.Close()
	}
	return out, "proc-net-sockets"
}

func decodeFlowSocketEndpoint(raw string) (string, string, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return "", "", false
	}
	port, err := strconv.ParseUint(parts[1], 16, 16)
	if err != nil {
		return "", "", false
	}
	host := parts[0]
	switch len(host) {
	case 8:
		bytes, err := hex.DecodeString(host)
		if err != nil || len(bytes) != 4 {
			return "", "", false
		}
		host = net.IPv4(bytes[3], bytes[2], bytes[1], bytes[0]).String()
	case 32:
		bytes, err := hex.DecodeString(host)
		if err != nil || len(bytes) != 16 {
			return "", "", false
		}
		for i := 0; i < 16; i += 4 {
			bytes[i], bytes[i+3] = bytes[i+3], bytes[i]
			bytes[i+1], bytes[i+2] = bytes[i+2], bytes[i+1]
		}
		host = net.IP(bytes).String()
	default:
		return "", "", false
	}
	return host, strconv.FormatUint(port, 10), true
}

func matchFlowSocket(entry flowExplorerEntry, sockets map[flowSocketKey]flowSocketRecord) flowSocketMatch {
	if entry.Protocol != "TCP" && entry.Protocol != "UDP" {
		return flowSocketMatch{}
	}
	candidates := []struct {
		Key   flowSocketKey
		Match string
	}{
		{
			Key: flowSocketKey{
				Protocol: entry.Protocol, LocalIP: entry.Original.Source, LocalPort: entry.Original.SourcePort,
				RemoteIP: entry.Original.Destination, RemotePort: entry.Original.DestinationPort,
			},
			Match: "original-outbound-local",
		},
		{
			Key: flowSocketKey{
				Protocol: entry.Protocol, LocalIP: entry.Original.Destination, LocalPort: entry.Original.DestinationPort,
				RemoteIP: entry.Original.Source, RemotePort: entry.Original.SourcePort,
			},
			Match: "original-inbound-local",
		},
		{
			Key: flowSocketKey{
				Protocol: entry.Protocol, LocalIP: entry.NAT.TranslatedDestination, LocalPort: entry.NAT.TranslatedDestinationPort,
				RemoteIP: entry.Original.Source, RemotePort: entry.Original.SourcePort,
			},
			Match: "dnat-inbound-local",
		},
		{
			Key: flowSocketKey{
				Protocol: entry.Protocol, LocalIP: entry.Reply.Source, LocalPort: entry.Reply.SourcePort,
				RemoteIP: entry.Reply.Destination, RemotePort: entry.Reply.DestinationPort,
			},
			Match: "reply-local",
		},
	}
	for _, candidate := range candidates {
		if candidate.Key.LocalIP == "" || candidate.Key.RemoteIP == "" ||
			candidate.Key.LocalPort == "" || candidate.Key.RemotePort == "" {
			continue
		}
		if record, ok := sockets[candidate.Key]; ok {
			return flowSocketMatch{Record: record, Match: candidate.Match}
		}
	}
	return flowSocketMatch{}
}

func resolveFlowSocketOwners(
	inodes map[string]bool,
	pidBudget int,
	fdBudget int,
) (map[string]flowSocketOwner, int, int, bool) {
	owners := map[string]flowSocketOwner{}
	if len(inodes) == 0 {
		return owners, 0, 0, false
	}
	if pidBudget <= 0 {
		pidBudget = flowOwnerPIDBudget
	}
	if fdBudget <= 0 {
		fdBudget = flowOwnerFDBudget
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return owners, 0, 0, false
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	pidsScanned := 0
	fdsScanned := 0
	budgetHit := false

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		if pidsScanned >= pidBudget {
			budgetHit = true
			break
		}
		pidsScanned++
		base := filepath.Join("/proc", entry.Name())
		process := strings.TrimSpace(readFlowText(filepath.Join(base, "comm")))
		fds, err := os.ReadDir(filepath.Join(base, "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			if fdsScanned >= fdBudget {
				budgetHit = true
				break
			}
			fdsScanned++
			target, err := os.Readlink(filepath.Join(base, "fd", fd.Name()))
			if err != nil || !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			if !inodes[inode] {
				continue
			}
			if _, exists := owners[inode]; !exists {
				owners[inode] = flowSocketOwner{PID: pid, Process: process}
			}
			if len(owners) == len(inodes) {
				return owners, pidsScanned, fdsScanned, budgetHit
			}
		}
		if budgetHit {
			break
		}
	}
	return owners, pidsScanned, fdsScanned, budgetHit
}

func readFlowText(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}

func readFlowPolicyRules(ctx context.Context) ([]policyRule, string) {
	if _, err := exec.LookPath("ip"); err != nil {
		return []policyRule{}, "unavailable"
	}
	raw, err := runIPCommand(ctx, "-4", "rule", "show")
	if err != nil {
		return []policyRule{}, "unavailable"
	}
	return parsePolicyOutput(raw, "ipv4"), "ip-rule"
}

func explainFlowPolicy(entry flowExplorerEntry, rules []policyRule) flowPolicyExplanation {
	result := flowPolicyExplanation{
		Available:  len(rules) > 0,
		Source:     "ip-rule",
		Mark:       entry.Mark,
		Candidates: []flowPolicyCandidate{},
	}
	if net.ParseIP(entry.Original.Source) == nil || net.ParseIP(entry.EffectiveDestination) == nil {
		result.Reason = "flow tuple is not suitable for IPv4 policy matching"
		return result
	}

	sorted := append([]policyRule(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Priority < sorted[j].Priority })
	for _, rule := range sorted {
		if rule.Family != "" && rule.Family != "ipv4" {
			continue
		}
		if !flowPolicyAddressMatches(rule.From, entry.Original.Source) {
			continue
		}
		if rule.To != "" && !flowPolicyAddressMatches(rule.To, entry.EffectiveDestination) {
			continue
		}
		if rule.Mark != "" && !flowPolicyMarkMatches(rule.Mark, entry.Mark) {
			continue
		}
		match := "matched-known-fields"
		if rule.IIF != "" || rule.OIF != "" {
			match = "candidate-interface-unknown"
		}
		result.Candidates = append(result.Candidates, flowPolicyCandidate{
			Priority: rule.Priority,
			Table:    rule.Table,
			From:     rule.From,
			To:       rule.To,
			Mark:     rule.Mark,
			IIF:      rule.IIF,
			OIF:      rule.OIF,
			Match:    match,
		})
		if len(result.Candidates) >= 16 {
			break
		}
	}
	if len(result.Candidates) == 0 {
		result.Reason = "no policy rule matched known flow fields"
	}
	return result
}

func flowPolicyAddressMatches(ruleValue, rawIP string) bool {
	ruleValue = strings.TrimSpace(ruleValue)
	if ruleValue == "" || ruleValue == "all" {
		return true
	}
	ip := net.ParseIP(rawIP)
	if ip == nil {
		return false
	}
	if networkIP, network, err := net.ParseCIDR(ruleValue); err == nil {
		_ = networkIP
		return network.Contains(ip)
	}
	return ip.Equal(net.ParseIP(ruleValue))
}

func flowPolicyMarkMatches(ruleMark, flowMark string) bool {
	ruleValue, ruleMask, ok := parseFlowMark(ruleMark)
	if !ok {
		return false
	}
	flowValue, _, ok := parseFlowMark(flowMark)
	if !ok {
		return false
	}
	return flowValue&ruleMask == ruleValue&ruleMask
}

func parseFlowMark(raw string) (uint64, uint64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, ^uint64(0), false
	}
	parts := strings.SplitN(raw, "/", 2)
	value, err := strconv.ParseUint(parts[0], 0, 64)
	if err != nil {
		return 0, 0, false
	}
	mask := ^uint64(0)
	if len(parts) == 2 {
		mask, err = strconv.ParseUint(parts[1], 0, 64)
		if err != nil {
			return 0, 0, false
		}
	}
	return value, mask, true
}

func normalizeFlowMark(raw string) string {
	value, _, ok := parseFlowMark(raw)
	if !ok {
		return "0"
	}
	return strconv.FormatUint(value, 10)
}

func flowKernelRouteGet(ctx context.Context, target, mark string) kernelRouteDecision {
	args := []string{"-4", "route", "get", target}
	if value, _, ok := parseFlowMark(mark); ok {
		args = append(args, "mark", strconv.FormatUint(value, 10))
	}
	raw, err := runIPCommand(ctx, args...)
	if err != nil {
		return kernelRouteDecision{
			Available: false,
			Family:    "ipv4",
			Error:     err.Error(),
			Raw:       strings.TrimSpace(raw),
		}
	}
	return parseRouteGet(raw, "ipv4")
}

func appendFlowReason(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}
