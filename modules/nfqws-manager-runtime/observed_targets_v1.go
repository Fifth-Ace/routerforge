package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	v2ObservedVersion          = 1
	v2ObservedMaxEntries       = 256
	v2ObservedMinEvents        = 2
	v2ObservedRetention        = 24 * time.Hour
	v2ObservedResolveBudget    = 24
	v2ObservedFlowLimit        = 256
	v2ObservedScanConfirm      = "ROUTERFORGE_OBSERVED_SCAN"
	v2ObservedIgnoreConfirm    = "ROUTERFORGE_OBSERVED_IGNORE"
	v2ObservedNetworkToolsSock = "/opt/var/run/routerforge-network-tools.sock"
)

var (
	v2ObservedRoot = v2TargetMemoryRoot
	v2ObservedPath = filepath.Join(v2ObservedRoot, "observed-targets.json")
	v2ObservedNow  = time.Now
)

type v2ObservedFlowTuple struct {
	Source          string `json:"source,omitempty"`
	Destination     string `json:"destination,omitempty"`
	SourcePort      string `json:"source_port,omitempty"`
	DestinationPort string `json:"destination_port,omitempty"`
	Packets         uint64 `json:"packets,omitempty"`
	Bytes           uint64 `json:"bytes,omitempty"`
}

type v2ObservedFlow struct {
	Protocol             string              `json:"protocol"`
	State                string              `json:"state"`
	TimeoutSeconds       int64               `json:"timeout_seconds,omitempty"`
	Original             v2ObservedFlowTuple `json:"original"`
	Reply                v2ObservedFlowTuple `json:"reply"`
	EffectiveDestination string              `json:"effective_destination,omitempty"`
}

type v2ObservedFlowEnvelope struct {
	Flows   []v2ObservedFlow `json:"flows"`
	Count   int              `json:"count"`
	Bounded bool             `json:"bounded"`
	Source  string           `json:"source"`
}

type v2ObservedTarget struct {
	Key         string `json:"key"`
	Device      string `json:"device"`
	Target      string `json:"target,omitempty"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	Port        int    `json:"port"`
	Reason      string `json:"reason"`
	Count       int    `json:"count"`
	FirstSeen   string `json:"first_seen"`
	LastSeen    string `json:"last_seen"`
	State       string `json:"state,omitempty"`
	Active      bool   `json:"active"`
}

type v2ObservedDocument struct {
	Version int                `json:"version"`
	Entries []v2ObservedTarget `json:"entries"`
	Ignored []string           `json:"ignored"`
}

type v2ObservedResponse struct {
	OK              bool               `json:"ok"`
	GeneratedAt     string             `json:"generated_at"`
	Observed        []v2ObservedTarget `json:"observed"`
	Count           int                `json:"count"`
	SuppressedCount int                `json:"suppressed_count"`
	IgnoredCount    int                `json:"ignored_count"`
	MinEvents       int                `json:"min_events"`
	RetentionSec    int64              `json:"retention_seconds"`
	FlowSource      string             `json:"flow_source,omitempty"`
	FlowCount       int                `json:"flow_count,omitempty"`
	MutationAPI     bool               `json:"mutation_api"`
	ProductionWrite bool               `json:"production_write"`
}

func registerObservedTargetsV1Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/observed-targets", getOnly(handleV2ObservedTargets))
	mux.HandleFunc("/v1/v2/observed-targets/scan", mutationOnly(handleV2ObservedTargetsScan))
	mux.HandleFunc("/v1/v2/observed-targets/ignore", mutationOnly(handleV2ObservedTargetsIgnore))
}

func v2ObservedDefaultDocument() v2ObservedDocument {
	return v2ObservedDocument{Version: v2ObservedVersion, Entries: []v2ObservedTarget{}, Ignored: []string{}}
}

func readV2ObservedDocument() (v2ObservedDocument, error) {
	doc := v2ObservedDefaultDocument()
	data, err := os.ReadFile(v2ObservedPath)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	if len(data) > 2<<20 {
		return doc, errors.New("observed-target store exceeds safety limit")
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, errors.New("decode observed-target store: " + err.Error())
	}
	if doc.Version != v2ObservedVersion {
		return doc, errors.New("unsupported observed-target store version")
	}
	if len(doc.Entries) > v2ObservedMaxEntries {
		return doc, errors.New("observed-target store exceeds entry limit")
	}
	if doc.Entries == nil {
		doc.Entries = []v2ObservedTarget{}
	}
	if doc.Ignored == nil {
		doc.Ignored = []string{}
	}
	return doc, nil
}

func writeV2ObservedDocument(doc v2ObservedDocument) error {
	if len(doc.Entries) > v2ObservedMaxEntries {
		return errors.New("observed-target entry limit exceeded")
	}
	if err := os.MkdirAll(v2ObservedRoot, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(v2ObservedRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("observed-target root is not a real directory")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return safety.WriteFileAtomic(v2ObservedPath, data, 0600)
}

func v2ObservedFlowKey(device, destination, protocol string, port int, reason string) string {
	return strings.Join([]string{
		strings.TrimSpace(device),
		strings.TrimSpace(destination),
		strings.ToUpper(strings.TrimSpace(protocol)),
		strconv.Itoa(port),
		strings.TrimSpace(reason),
	}, "\x00")
}

func v2ObservedPublicDestination(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return ip != nil && ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast()
}

func v2ObservedLANSource(raw string) bool {
	ip := net.ParseIP(strings.TrimSpace(raw))
	return ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast()) && !ip.IsLoopback()
}

func v2ObservedPortSet(config, protocol string) map[int]bool {
	protocol = strings.ToUpper(strings.TrimSpace(protocol))
	prefix := "--filter-" + strings.ToLower(protocol) + "="
	out := map[int]bool{}
	for _, token := range strings.Fields(strings.NewReplacer("\"", " ", "'", " ", "\r", " ", "\n", " ").Replace(config)) {
		if strings.HasPrefix(token, prefix) {
			v2ObservedAddPortSpec(out, strings.TrimPrefix(token, prefix))
		}
	}
	assignment := "TCP_PORTS="
	if protocol == "UDP" {
		assignment = "UDP_PORTS="
	}
	for _, line := range strings.Split(config, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, assignment) {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, assignment)), "\"'")
		v2ObservedAddPortSpec(out, raw)
	}
	if len(out) == 0 {
		if protocol == "TCP" {
			out[80], out[443] = true, true
		} else if protocol == "UDP" {
			out[443] = true
		}
	}
	return out
}

func v2ObservedAddPortSpec(out map[int]bool, raw string) {
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") || strings.Contains(part, ":") {
			sep := "-"
			if strings.Contains(part, ":") {
				sep = ":"
			}
			bits := strings.SplitN(part, sep, 2)
			if len(bits) != 2 {
				continue
			}
			a, errA := strconv.Atoi(strings.TrimSpace(bits[0]))
			b, errB := strconv.Atoi(strings.TrimSpace(bits[1]))
			if errA != nil || errB != nil || a < 1 || b < a || b > 65535 || b-a > 4096 {
				continue
			}
			for p := a; p <= b; p++ {
				out[p] = true
			}
			continue
		}
		if p, err := strconv.Atoi(part); err == nil && p >= 1 && p <= 65535 {
			out[p] = true
		}
	}
}

func v2ObservedClassify(flow v2ObservedFlow, config string) (v2ObservedTarget, bool) {
	protocol := strings.ToUpper(strings.TrimSpace(flow.Protocol))
	if protocol != "TCP" && protocol != "UDP" {
		return v2ObservedTarget{}, false
	}
	device := strings.TrimSpace(flow.Original.Source)
	destination := strings.TrimSpace(flow.EffectiveDestination)
	if destination == "" {
		destination = strings.TrimSpace(flow.Original.Destination)
	}
	if !v2ObservedLANSource(device) || !v2ObservedPublicDestination(destination) {
		return v2ObservedTarget{}, false
	}
	port, err := strconv.Atoi(strings.TrimSpace(flow.Original.DestinationPort))
	if err != nil || port < 1 || port > 65535 {
		return v2ObservedTarget{}, false
	}
	if !v2ObservedPortSet(config, protocol)[port] {
		return v2ObservedTarget{}, false
	}

	state := strings.ToUpper(strings.TrimSpace(flow.State))
	reason := ""
	switch protocol {
	case "TCP":
		switch {
		case state == "SYN_SENT" && flow.Original.Packets >= 2 && flow.Reply.Packets == 0:
			reason = "tcp_syn_no_reply"
		case state == "UNREPLIED" && flow.Original.Packets >= 2 && flow.Reply.Packets == 0:
			reason = "tcp_unreplied"
		case state == "CLOSE" && flow.Original.Packets > 0 && flow.Reply.Packets <= 1:
			reason = "tcp_rst_or_early_close"
		}
	case "UDP":
		if state == "UNREPLIED" && flow.Original.Packets >= 2 && flow.Reply.Packets == 0 &&
			flow.TimeoutSeconds > 0 && flow.TimeoutSeconds <= 15 {
			reason = "udp_no_reply"
		}
	}
	if reason == "" {
		return v2ObservedTarget{}, false
	}
	return v2ObservedTarget{
		Key:         v2ObservedFlowKey(device, destination, protocol, port, reason),
		Device:      device,
		Destination: destination,
		Protocol:    protocol,
		Port:        port,
		Reason:      reason,
		State:       state,
		Active:      true,
	}, true
}

func v2ObservedNetworkClient() *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", v2ObservedNetworkToolsSock)
		},
	}
	return &http.Client{Transport: transport, Timeout: 6 * time.Second}
}

func v2ObservedReadFlows(ctx context.Context) (v2ObservedFlowEnvelope, error) {
	client := v2ObservedNetworkClient()
	defer client.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/v1/flow-explorer?limit=256", nil)
	if err != nil {
		return v2ObservedFlowEnvelope{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return v2ObservedFlowEnvelope{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return v2ObservedFlowEnvelope{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return v2ObservedFlowEnvelope{}, errors.New("network-tools flow explorer returned HTTP " + strconv.Itoa(resp.StatusCode))
	}
	var envelope v2ObservedFlowEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return envelope, errors.New("decode network-tools flow explorer: " + err.Error())
	}
	if len(envelope.Flows) > v2ObservedFlowLimit {
		envelope.Flows = envelope.Flows[:v2ObservedFlowLimit]
	}
	return envelope, nil
}

func v2ObservedCorrelation(ctx context.Context) map[string]string {
	out := map[string]string{}
	if tcp16, err := readV2TCP16Memory(); err == nil {
		for _, entry := range tcp16.Entries {
			if entry.DestinationIPv4 != "" && entry.WorkingSNI != "" {
				out[entry.DestinationIPv4] = entry.WorkingSNI
			}
		}
	}
	memory, err := readV2TargetMemory()
	if err != nil {
		return out
	}
	sort.SliceStable(memory.Entries, func(i, j int) bool {
		return memory.Entries[i].LastVerified > memory.Entries[j].LastVerified
	})
	seen := map[string]bool{}
	targets := []string{}
	for _, entry := range memory.Entries {
		target := strings.TrimSpace(entry.Target)
		if target == "" || seen[target] || net.ParseIP(target) != nil {
			continue
		}
		seen[target] = true
		targets = append(targets, target)
		if len(targets) >= v2ObservedResolveBudget {
			break
		}
	}
	for _, target := range targets {
		addresses, lookupErr := net.DefaultResolver.LookupIPAddr(ctx, target)
		if lookupErr != nil {
			continue
		}
		for _, address := range addresses {
			if ip := address.IP.To4(); ip != nil {
				if _, exists := out[ip.String()]; !exists {
					out[ip.String()] = target
				}
			}
		}
	}
	return out
}

func v2ObservedIgnored(doc v2ObservedDocument, item v2ObservedTarget) bool {
	for _, ignored := range doc.Ignored {
		ignored = strings.TrimSpace(strings.ToLower(ignored))
		if ignored == "" {
			continue
		}
		if strings.EqualFold(ignored, item.Destination) || (item.Target != "" && strings.EqualFold(ignored, item.Target)) {
			return true
		}
	}
	return false
}

func v2ObservedMergeScan(doc v2ObservedDocument, flows []v2ObservedFlow, config string, correlation map[string]string, now time.Time) v2ObservedDocument {
	now = now.UTC()
	cutoff := now.Add(-v2ObservedRetention)
	existing := map[string]int{}
	wasActive := map[string]bool{}
	filtered := make([]v2ObservedTarget, 0, len(doc.Entries))
	for _, entry := range doc.Entries {
		stamp, err := time.Parse(time.RFC3339, entry.LastSeen)
		if err == nil && stamp.Before(cutoff) {
			continue
		}
		wasActive[entry.Key] = entry.Active
		entry.Active = false
		existing[entry.Key] = len(filtered)
		filtered = append(filtered, entry)
	}
	doc.Entries = filtered

	stamp := now.Format(time.RFC3339)
	for _, flow := range flows {
		item, ok := v2ObservedClassify(flow, config)
		if !ok {
			continue
		}
		if target := strings.TrimSpace(correlation[item.Destination]); target != "" {
			item.Target = target
		}
		if idx, exists := existing[item.Key]; exists {
			entry := doc.Entries[idx]
			if !wasActive[item.Key] {
				entry.Count++
			}
			entry.LastSeen = stamp
			entry.State = item.State
			entry.Active = true
			if item.Target != "" {
				entry.Target = item.Target
			}
			doc.Entries[idx] = entry
			continue
		}
		item.Count = 1
		item.FirstSeen = stamp
		item.LastSeen = stamp
		doc.Entries = append(doc.Entries, item)
		existing[item.Key] = len(doc.Entries) - 1
	}
	sort.SliceStable(doc.Entries, func(i, j int) bool {
		if doc.Entries[i].Count != doc.Entries[j].Count {
			return doc.Entries[i].Count > doc.Entries[j].Count
		}
		return doc.Entries[i].LastSeen > doc.Entries[j].LastSeen
	})
	if len(doc.Entries) > v2ObservedMaxEntries {
		doc.Entries = doc.Entries[:v2ObservedMaxEntries]
	}
	return doc
}

func v2ObservedResponseFor(doc v2ObservedDocument, flowSource string, flowCount int) v2ObservedResponse {
	visible := []v2ObservedTarget{}
	suppressed := 0
	for _, entry := range doc.Entries {
		if entry.Count < v2ObservedMinEvents || v2ObservedIgnored(doc, entry) {
			suppressed++
			continue
		}
		visible = append(visible, entry)
	}
	return v2ObservedResponse{
		OK:              true,
		GeneratedAt:     v2ObservedNow().UTC().Format(time.RFC3339),
		Observed:        visible,
		Count:           len(visible),
		SuppressedCount: suppressed,
		IgnoredCount:    len(doc.Ignored),
		MinEvents:       v2ObservedMinEvents,
		RetentionSec:    int64(v2ObservedRetention / time.Second),
		FlowSource:      flowSource,
		FlowCount:       flowCount,
		MutationAPI:     false,
		ProductionWrite: false,
	}
}

func handleV2ObservedTargets(w http.ResponseWriter, _ *http.Request) {
	doc, err := readV2ObservedDocument()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v2ObservedResponseFor(doc, "", 0))
}

func handleV2ObservedTargetsScan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid observed-target scan request"})
		return
	}
	if req.Confirm != v2ObservedScanConfirm {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm must equal " + v2ObservedScanConfirm})
		return
	}
	statusBefore := readStatus()
	configSHA := statusBefore.ConfigSHA256
	ctx, cancel := context.WithTimeout(r.Context(), 7*time.Second)
	defer cancel()

	envelope, err := v2ObservedReadFlows(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":            "read Network Tools flow data: " + err.Error(),
			"production_write": false,
		})
		return
	}
	correlation := v2ObservedCorrelation(ctx)
	doc, err := readV2ObservedDocument()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	doc = v2ObservedMergeScan(doc, envelope.Flows, statusBefore.Config, correlation, v2ObservedNow())

	if current := readStatus().ConfigSHA256; !strings.EqualFold(current, configSHA) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":            "production config changed during observed-target scan; evidence was not persisted",
			"production_write": false,
		})
		return
	}
	if err := writeV2ObservedDocument(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "persist observed-target evidence: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, v2ObservedResponseFor(doc, envelope.Source, len(envelope.Flows)))
}

func handleV2ObservedTargetsIgnore(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target  string `json:"target"`
		Ignore  *bool  `json:"ignore"`
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid observed-target ignore request"})
		return
	}
	if req.Confirm != v2ObservedIgnoreConfirm {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm must equal " + v2ObservedIgnoreConfirm})
		return
	}
	target := strings.ToLower(strings.TrimSpace(req.Target))
	if target == "" || len(target) > 253 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "target is required"})
		return
	}
	ignore := true
	if req.Ignore != nil {
		ignore = *req.Ignore
	}
	doc, err := readV2ObservedDocument()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	set := map[string]bool{}
	for _, item := range doc.Ignored {
		item = strings.ToLower(strings.TrimSpace(item))
		if item != "" {
			set[item] = true
		}
	}
	if ignore {
		set[target] = true
	} else {
		delete(set, target)
	}
	doc.Ignored = doc.Ignored[:0]
	for item := range set {
		doc.Ignored = append(doc.Ignored, item)
	}
	sort.Strings(doc.Ignored)
	if err := writeV2ObservedDocument(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "target": target, "ignored": ignore, "ignored_count": len(doc.Ignored),
		"production_write": false,
	})
}
