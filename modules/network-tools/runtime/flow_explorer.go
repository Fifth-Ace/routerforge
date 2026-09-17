package main

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const flowExplorerMaxLimit = 256

type flowTuple struct {
	Source          string `json:"source,omitempty"`
	Destination     string `json:"destination,omitempty"`
	SourcePort      string `json:"source_port,omitempty"`
	DestinationPort string `json:"destination_port,omitempty"`
	Packets         uint64 `json:"packets,omitempty"`
	Bytes           uint64 `json:"bytes,omitempty"`
}

type flowRouteExplanation struct {
	Available   bool   `json:"available"`
	Source      string `json:"source"`
	Target      string `json:"target,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	Destination string `json:"destination,omitempty"`
	Prefix      int    `json:"prefix,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Table       string `json:"table,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type flowNATExplanation struct {
	Detected                  bool     `json:"detected"`
	Types                     []string `json:"types"`
	TranslatedSource          string   `json:"translated_source,omitempty"`
	TranslatedSourcePort      string   `json:"translated_source_port,omitempty"`
	TranslatedDestination     string   `json:"translated_destination,omitempty"`
	TranslatedDestinationPort string   `json:"translated_destination_port,omitempty"`
}

type flowExplorerEntry struct {
	Protocol             string               `json:"protocol"`
	State                string               `json:"state"`
	TimeoutSeconds       int64                `json:"timeout_seconds,omitempty"`
	Original             flowTuple            `json:"original"`
	Reply                flowTuple            `json:"reply"`
	NAT                  flowNATExplanation   `json:"nat"`
	EffectiveDestination string               `json:"effective_destination,omitempty"`
	Route                flowRouteExplanation `json:"route"`
}

type flowExplorerFilter struct {
	Protocol    string
	Source      string
	Destination string
}

func handleFlowExplorer(w http.ResponseWriter, r *http.Request) {
	limit := 128
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 || value > flowExplorerMaxLimit {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "limit must be between 1 and 256",
			})
			return
		}
		limit = value
	}

	filter := flowExplorerFilter{
		Protocol:    strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("protocol"))),
		Source:      strings.TrimSpace(r.URL.Query().Get("source")),
		Destination: strings.TrimSpace(r.URL.Query().Get("destination")),
	}
	if filter.Protocol != "" &&
		filter.Protocol != "TCP" &&
		filter.Protocol != "UDP" &&
		filter.Protocol != "ICMP" &&
		filter.Protocol != "ICMPV6" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "protocol must be TCP, UDP, ICMP or ICMPV6",
		})
		return
	}
	if filter.Source != "" && net.ParseIP(filter.Source) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "source must be an IP address"})
		return
	}
	if filter.Destination != "" && net.ParseIP(filter.Destination) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "destination must be an IP address"})
		return
	}

	entries, source := readFlowExplorer(limit, filter)
	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC(),
		"flows":        entries,
		"count":        len(entries),
		"limit":        limit,
		"bounded":      true,
		"sampling":     "first-matching-conntrack-entries",
		"source":       source,
		"mutation_api": false,
		"dpi":          false,
	})
}

func readFlowExplorer(limit int, filter flowExplorerFilter) ([]flowExplorerEntry, string) {
	if limit <= 0 || limit > flowExplorerMaxLimit {
		limit = 128
	}
	routes, _ := readRoutes()
	paths := []string{"/proc/net/nf_conntrack", "/proc/net/ip_conntrack"}

	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer file.Close()

		out := make([]flowExplorerEntry, 0, minInt(limit, 64))
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 64<<10)
		for scanner.Scan() && len(out) < limit {
			entry, ok := parseFlowExplorer(scanner.Text())
			if !ok || !flowExplorerMatches(entry, filter) {
				continue
			}
			entry.Route = explainFlowRoute(entry, routes)
			out = append(out, entry)
		}
		return out, path
	}

	return []flowExplorerEntry{}, "unavailable"
}

func parseFlowExplorer(line string) (flowExplorerEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return flowExplorerEntry{}, false
	}

	entry := flowExplorerEntry{
		NAT: flowNATExplanation{Types: []string{}},
	}
	tuple := &entry.Original
	srcCount := 0
	timeoutCaptured := false

	for i, field := range fields {
		lower := strings.ToLower(field)
		if entry.Protocol == "" &&
			(lower == "tcp" || lower == "udp" || lower == "icmp" || lower == "icmpv6") {
			entry.Protocol = strings.ToUpper(lower)
			if i+2 < len(fields) {
				if timeout, err := strconv.ParseInt(fields[i+2], 10, 64); err == nil {
					entry.TimeoutSeconds = timeout
					timeoutCaptured = true
				}
			}
			continue
		}
		if entry.State == "" && isConntrackState(field) {
			entry.State = field
			continue
		}

		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		if key == "src" {
			srcCount++
			if srcCount >= 2 {
				tuple = &entry.Reply
			}
		}
		switch key {
		case "src":
			tuple.Source = value
		case "dst":
			tuple.Destination = value
		case "sport":
			tuple.SourcePort = value
		case "dport":
			tuple.DestinationPort = value
		case "packets":
			if tuple.Packets == 0 {
				tuple.Packets, _ = strconv.ParseUint(value, 10, 64)
			}
		case "bytes":
			if tuple.Bytes == 0 {
				tuple.Bytes, _ = strconv.ParseUint(value, 10, 64)
			}
		}
	}

	if !timeoutCaptured {
		entry.TimeoutSeconds = 0
	}
	if entry.State == "" {
		entry.State = "ACTIVE"
	}
	if entry.Protocol == "" || entry.Original.Source == "" || entry.Original.Destination == "" {
		return flowExplorerEntry{}, false
	}

	entry.NAT = inferFlowNAT(entry.Original, entry.Reply)
	entry.EffectiveDestination = entry.Original.Destination
	if entry.NAT.TranslatedDestination != "" {
		entry.EffectiveDestination = entry.NAT.TranslatedDestination
	}
	return entry, true
}

func isConntrackState(value string) bool {
	switch value {
	case "ESTABLISHED", "TIME_WAIT", "SYN_SENT", "SYN_RECV", "FIN_WAIT",
		"CLOSE_WAIT", "LAST_ACK", "CLOSE", "LISTEN", "UNREPLIED", "ASSURED":
		return true
	default:
		return false
	}
}

func inferFlowNAT(original, reply flowTuple) flowNATExplanation {
	nat := flowNATExplanation{Types: []string{}}
	if reply.Source != "" &&
		(original.Destination != reply.Source ||
			(original.DestinationPort != "" && reply.SourcePort != "" && original.DestinationPort != reply.SourcePort)) {
		nat.Detected = true
		nat.Types = append(nat.Types, "DNAT")
		nat.TranslatedDestination = reply.Source
		nat.TranslatedDestinationPort = reply.SourcePort
	}
	if reply.Destination != "" &&
		(original.Source != reply.Destination ||
			(original.SourcePort != "" && reply.DestinationPort != "" && original.SourcePort != reply.DestinationPort)) {
		nat.Detected = true
		nat.Types = append(nat.Types, "SNAT")
		nat.TranslatedSource = reply.Destination
		nat.TranslatedSourcePort = reply.DestinationPort
	}
	sort.Strings(nat.Types)
	return nat
}

func explainFlowRoute(entry flowExplorerEntry, routes []routeEntry) flowRouteExplanation {
	target := strings.TrimSpace(entry.EffectiveDestination)
	result := flowRouteExplanation{
		Available: false,
		Source:    "kernel-main-table",
		Target:    target,
	}
	ip := net.ParseIP(target)
	if ip == nil {
		result.Reason = "effective destination is not an IP address"
		return result
	}
	if ip.To4() == nil {
		result.Source = "unavailable-ipv6"
		result.Reason = "P20A route correlation currently uses /proc/net/route IPv4 main table"
		return result
	}
	route := selectRoute(routes, ip.To4())
	if route == nil {
		result.Reason = "no matching IPv4 main-table route"
		return result
	}
	result.Available = true
	result.Interface = route.Interface
	result.Gateway = route.Gateway
	result.Destination = route.Destination
	result.Prefix = route.Prefix
	result.Metric = route.Metric
	result.Table = route.Table
	return result
}

func flowExplorerMatches(entry flowExplorerEntry, filter flowExplorerFilter) bool {
	if filter.Protocol != "" && entry.Protocol != filter.Protocol {
		return false
	}
	if filter.Source != "" &&
		entry.Original.Source != filter.Source &&
		entry.Reply.Destination != filter.Source &&
		entry.NAT.TranslatedSource != filter.Source {
		return false
	}
	if filter.Destination != "" &&
		entry.Original.Destination != filter.Destination &&
		entry.Reply.Source != filter.Destination &&
		entry.NAT.TranslatedDestination != filter.Destination {
		return false
	}
	return true
}
