package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	errDNSResolverNotFound = errors.New("DNS resolver not found")
	errDNSResolverReadOnly = errors.New("DNS resolver is read-only")
	errDNSResolverConflict = errors.New("DNS resolver conflicts with an existing resolver")
	errDNSResolverInvalid  = errors.New("invalid DNS resolver")
)

const (
	dnsDisabledStoreVersion  = 1
	dnsKeeneticDoTSlotLimit = 8
)

var dnsSafeToken = regexp.MustCompile(`^[A-Za-z0-9._:@/\-+*=]{0,512}$`)

type DNSResolverSpec struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name,omitempty"`
	Protocol       string   `json:"protocol"` // DNS | DoT | DoH
	Address        string   `json:"address,omitempty"`
	URI            string   `json:"uri,omitempty"`
	Port           int      `json:"port,omitempty"`
	SNI            string   `json:"sni,omitempty"`
	Interface      string   `json:"interface,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	SPKI           string   `json:"spki,omitempty"`
	Format         string   `json:"format,omitempty"`
	Disabled       bool     `json:"disabled,omitempty"`
	Dynamic        bool     `json:"dynamic,omitempty"`
	Service        string   `json:"service,omitempty"`
	Source         string   `json:"source,omitempty"`
	PhysicalCount  int      `json:"physical_count,omitempty"`
	ReadOnlyReason string   `json:"read_only_reason,omitempty"`
}

type DNSResolverList struct {
	Resolvers          []DNSResolverSpec `json:"resolvers"`
	ActiveCount        int               `json:"active_count"`
	DisabledCount      int               `json:"disabled_count"`
	DynamicCount       int               `json:"dynamic_count"`
	MutationAPI        bool              `json:"mutation_api"`
	NativeMode         bool              `json:"native_mode"`
	GeneratedSlots     int               `json:"physical_entries"`
	DoTPhysicalEntries int               `json:"dot_physical_entries"`
	DoTPhysicalLimit   int               `json:"dot_physical_limit"`
}

type DNSResolverPreview struct {
	Resolver        DNSResolverSpec  `json:"resolver"`
	PhysicalEntries []map[string]any `json:"physical_entries"`
	PhysicalCount   int              `json:"physical_count"`
}

type DNSMutationResult struct {
	OK       bool             `json:"ok"`
	Action   string           `json:"action"`
	Resolver *DNSResolverSpec `json:"resolver,omitempty"`
	Rollback bool             `json:"rollback,omitempty"`
	Message  string           `json:"message,omitempty"`
}

type dnsLogicalResolver struct {
	Spec       DNSResolverSpec
	RawEntries []map[string]any
}

type dnsActiveNameServer struct {
	Address   string `json:"address"`
	Domain    string `json:"domain"`
	Global    int    `json:"global"`
	Service   string `json:"service"`
	Interface string `json:"interface"`
}

// Keenetic firmware does not expose one stable JSON shape for
// /show/sc/ip/name-server. An empty saved configuration is an array on the
// Hopper used for hardware validation, while other RCI endpoints/firmware can
// expose a {"server":[...]} envelope. Keep both forms inside the DNS module so
// Core and the Module ABI do not need firmware-specific knowledge.
type dnsSavedNameServers struct {
	Server []map[string]any
}

func (s *dnsSavedNameServers) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return fmt.Errorf("empty saved name-server response")
	}
	if raw == "null" {
		s.Server = nil
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var items []map[string]any
		if err := json.Unmarshal(data, &items); err != nil {
			return err
		}
		s.Server = items
		return nil
	}
	var envelope struct {
		Server []map[string]any `json:"server"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return err
	}
	s.Server = envelope.Server
	return nil
}

type dnsConfigState struct {
	TLS           []map[string]any
	HTTPS         []map[string]any
	Plain         []map[string]any
	Active        []dnsActiveNameServer
	Logical       map[string]*dnsLogicalResolver
	Dynamic       []DNSResolverSpec
	PlainReadable bool
}

type dnsDisabledRecord struct {
	Resolver   DNSResolverSpec  `json:"resolver"`
	RawEntries []map[string]any `json:"raw_entries,omitempty"`
}

type dnsDisabledStore struct {
	Version   int                 `json:"version"`
	Resolvers []dnsDisabledRecord `json:"resolvers"`
}

type dnsControlManager struct {
	mu           sync.Mutex
	rci          *dnsRCIClient
	disabledPath string
}

func newDNSControlManager(rci *dnsRCIClient, disabledPath string) *dnsControlManager {
	return &dnsControlManager{rci: rci, disabledPath: disabledPath}
}

func previewDNSResolver(spec DNSResolverSpec) (DNSResolverPreview, error) {
	normalized, err := normalizeDNSResolverSpec(spec)
	if err != nil {
		return DNSResolverPreview{}, err
	}
	entries, err := buildDNSResolverEntries(normalized)
	if err != nil {
		return DNSResolverPreview{}, err
	}
	normalized.ID = dnsResolverID(normalized)
	normalized.PhysicalCount = len(entries)
	normalized.Source = "static"
	return DNSResolverPreview{Resolver: normalized, PhysicalEntries: entries, PhysicalCount: len(entries)}, nil
}

func (m *dnsControlManager) List(ctx context.Context) (DNSResolverList, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState(ctx)
	if err != nil {
		return DNSResolverList{}, err
	}
	disabled, err := m.loadDisabled()
	if err != nil {
		return DNSResolverList{}, err
	}
	out := DNSResolverList{
		MutationAPI:      true,
		NativeMode:       true,
		DoTPhysicalLimit: dnsKeeneticDoTSlotLimit,
	}
	ids := make([]string, 0, len(state.Logical))
	for id := range state.Logical {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		spec := state.Logical[id].Spec
		out.Resolvers = append(out.Resolvers, spec)
		out.ActiveCount++
		out.GeneratedSlots += spec.PhysicalCount
		if spec.Protocol == "DoT" {
			out.DoTPhysicalEntries += spec.PhysicalCount
		}
	}
	for _, spec := range state.Dynamic {
		out.Resolvers = append(out.Resolvers, spec)
		out.DynamicCount++
	}
	for _, record := range disabled.Resolvers {
		spec := record.Resolver
		spec.Disabled = true
		spec.Source = "disabled"
		out.Resolvers = append(out.Resolvers, spec)
		out.DisabledCount++
	}
	sort.SliceStable(out.Resolvers, func(i, j int) bool {
		if out.Resolvers[i].Disabled != out.Resolvers[j].Disabled {
			return !out.Resolvers[i].Disabled
		}
		if out.Resolvers[i].Dynamic != out.Resolvers[j].Dynamic {
			return !out.Resolvers[i].Dynamic
		}
		if out.Resolvers[i].Protocol != out.Resolvers[j].Protocol {
			return out.Resolvers[i].Protocol < out.Resolvers[j].Protocol
		}
		return strings.ToLower(out.Resolvers[i].Name) < strings.ToLower(out.Resolvers[j].Name)
	})
	return out, nil
}

func (m *dnsControlManager) Create(ctx context.Context, spec DNSResolverSpec) (DNSMutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	normalized, err := normalizeDNSResolverSpec(spec)
	if err != nil {
		return DNSMutationResult{}, err
	}
	state, err := m.loadState(ctx)
	if err != nil {
		return DNSMutationResult{}, err
	}
	disabled, err := m.loadDisabled()
	if err != nil {
		return DNSMutationResult{}, err
	}
	normalized.ID = dnsResolverID(normalized)
	if _, exists := state.Logical[normalized.ID]; exists || disabledHasID(disabled, normalized.ID) {
		return DNSMutationResult{}, fmt.Errorf("%w: %s", errDNSResolverConflict, normalized.ID)
	}
	entries, err := buildDNSResolverEntries(normalized)
	if err != nil {
		return DNSMutationResult{}, err
	}
	normalized.PhysicalCount = len(entries)
	normalized.Source = "static"
	state.Logical[normalized.ID] = &dnsLogicalResolver{Spec: normalized, RawEntries: entries}
	if err := m.applyMutation(ctx, state, map[string]bool{normalized.Protocol: true}, nil); err != nil {
		return DNSMutationResult{}, err
	}
	return DNSMutationResult{OK: true, Action: "create", Resolver: &normalized}, nil
}

func (m *dnsControlManager) Update(ctx context.Context, id string, spec DNSResolverSpec) (DNSMutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState(ctx)
	if err != nil {
		return DNSMutationResult{}, err
	}
	disabled, err := m.loadDisabled()
	if err != nil {
		return DNSMutationResult{}, err
	}
	if dynamicByID(state.Dynamic, id) != nil {
		return DNSMutationResult{}, fmt.Errorf("%w: dynamic DHCP/service resolver", errDNSResolverReadOnly)
	}
	if index := disabledIndex(disabled, id); index >= 0 {
		normalized, err := normalizeDNSResolverSpec(spec)
		if err != nil {
			return DNSMutationResult{}, err
		}
		normalized.ID = dnsResolverID(normalized)
		normalized.Disabled = true
		normalized.Source = "disabled"
		entries, err := buildDNSResolverEntries(normalized)
		if err != nil {
			return DNSMutationResult{}, err
		}
		normalized.PhysicalCount = len(entries)
		if normalized.ID != id && (disabledHasID(disabled, normalized.ID) || state.Logical[normalized.ID] != nil) {
			return DNSMutationResult{}, fmt.Errorf("%w: %s", errDNSResolverConflict, normalized.ID)
		}
		disabled.Resolvers[index] = dnsDisabledRecord{Resolver: normalized, RawEntries: entries}
		if err := m.saveDisabled(disabled); err != nil {
			return DNSMutationResult{}, err
		}
		return DNSMutationResult{OK: true, Action: "update", Resolver: &normalized}, nil
	}
	current := state.Logical[id]
	if current == nil {
		return DNSMutationResult{}, errDNSResolverNotFound
	}
	normalized, err := normalizeDNSResolverSpec(spec)
	if err != nil {
		return DNSMutationResult{}, err
	}
	normalized.ID = dnsResolverID(normalized)
	if normalized.ID != id {
		if state.Logical[normalized.ID] != nil || disabledHasID(disabled, normalized.ID) {
			return DNSMutationResult{}, fmt.Errorf("%w: %s", errDNSResolverConflict, normalized.ID)
		}
	}
	entries, err := buildDNSResolverEntries(normalized)
	if err != nil {
		return DNSMutationResult{}, err
	}
	normalized.PhysicalCount = len(entries)
	normalized.Source = "static"
	delete(state.Logical, id)
	state.Logical[normalized.ID] = &dnsLogicalResolver{Spec: normalized, RawEntries: entries}
	changed := map[string]bool{current.Spec.Protocol: true, normalized.Protocol: true}
	if err := m.applyMutation(ctx, state, changed, nil); err != nil {
		return DNSMutationResult{}, err
	}
	return DNSMutationResult{OK: true, Action: "update", Resolver: &normalized}, nil
}

func (m *dnsControlManager) Delete(ctx context.Context, id string) (DNSMutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState(ctx)
	if err != nil {
		return DNSMutationResult{}, err
	}
	disabled, err := m.loadDisabled()
	if err != nil {
		return DNSMutationResult{}, err
	}
	if dynamicByID(state.Dynamic, id) != nil {
		return DNSMutationResult{}, fmt.Errorf("%w: dynamic DHCP/service resolver", errDNSResolverReadOnly)
	}
	if index := disabledIndex(disabled, id); index >= 0 {
		spec := disabled.Resolvers[index].Resolver
		disabled.Resolvers = append(disabled.Resolvers[:index], disabled.Resolvers[index+1:]...)
		if err := m.saveDisabled(disabled); err != nil {
			return DNSMutationResult{}, err
		}
		return DNSMutationResult{OK: true, Action: "delete", Resolver: &spec}, nil
	}
	current := state.Logical[id]
	if current == nil {
		return DNSMutationResult{}, errDNSResolverNotFound
	}
	delete(state.Logical, id)
	if err := m.applyMutation(ctx, state, map[string]bool{current.Spec.Protocol: true}, nil); err != nil {
		return DNSMutationResult{}, err
	}
	spec := current.Spec
	return DNSMutationResult{OK: true, Action: "delete", Resolver: &spec}, nil
}

func (m *dnsControlManager) Disable(ctx context.Context, id string) (DNSMutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState(ctx)
	if err != nil {
		return DNSMutationResult{}, err
	}
	if dynamicByID(state.Dynamic, id) != nil {
		return DNSMutationResult{}, fmt.Errorf("%w: dynamic DHCP/service resolver", errDNSResolverReadOnly)
	}
	current := state.Logical[id]
	if current == nil {
		return DNSMutationResult{}, errDNSResolverNotFound
	}
	disabled, err := m.loadDisabled()
	if err != nil {
		return DNSMutationResult{}, err
	}
	if disabledHasID(disabled, id) {
		return DNSMutationResult{}, fmt.Errorf("%w: resolver is already disabled", errDNSResolverConflict)
	}
	beforeDisabled := cloneDisabledStore(disabled)
	spec := current.Spec
	spec.Disabled = true
	spec.Source = "disabled"
	disabled.Resolvers = append(disabled.Resolvers, dnsDisabledRecord{Resolver: spec, RawEntries: cloneMapSlice(current.RawEntries)})
	delete(state.Logical, id)
	if err := m.applyMutation(ctx, state, map[string]bool{current.Spec.Protocol: true}, func() error {
		return m.saveDisabled(disabled)
	}); err != nil {
		_ = m.saveDisabled(beforeDisabled)
		return DNSMutationResult{}, err
	}
	return DNSMutationResult{OK: true, Action: "disable", Resolver: &spec}, nil
}

func (m *dnsControlManager) Enable(ctx context.Context, id string) (DNSMutationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	state, err := m.loadState(ctx)
	if err != nil {
		return DNSMutationResult{}, err
	}
	disabled, err := m.loadDisabled()
	if err != nil {
		return DNSMutationResult{}, err
	}
	index := disabledIndex(disabled, id)
	if index < 0 {
		return DNSMutationResult{}, errDNSResolverNotFound
	}
	record := disabled.Resolvers[index]
	spec := record.Resolver
	spec.Disabled = false
	spec.Source = "static"
	if state.Logical[spec.ID] != nil {
		return DNSMutationResult{}, fmt.Errorf("%w: active resolver %s already exists", errDNSResolverConflict, spec.ID)
	}
	entries := cloneMapSlice(record.RawEntries)
	if len(entries) == 0 {
		entries, err = buildDNSResolverEntries(spec)
		if err != nil {
			return DNSMutationResult{}, err
		}
	}
	spec.PhysicalCount = len(entries)
	state.Logical[spec.ID] = &dnsLogicalResolver{Spec: spec, RawEntries: entries}
	beforeDisabled := cloneDisabledStore(disabled)
	disabled.Resolvers = append(disabled.Resolvers[:index], disabled.Resolvers[index+1:]...)
	if err := m.applyMutation(ctx, state, map[string]bool{spec.Protocol: true}, func() error {
		return m.saveDisabled(disabled)
	}); err != nil {
		_ = m.saveDisabled(beforeDisabled)
		return DNSMutationResult{}, err
	}
	return DNSMutationResult{OK: true, Action: "enable", Resolver: &spec}, nil
}

// applyMutation snapshots the current native sections, applies only affected
// protocol sections, saves, verifies RCI readback and rolls the native sections
// back on any mismatch. afterVerify runs only after native verification and is
// used for RouterForge-only metadata such as temporarily disabled resolvers.
func (m *dnsControlManager) applyMutation(ctx context.Context, desired *dnsConfigState, changed map[string]bool, afterVerify func() error) error {
	if err := validateDNSPhysicalLimits(desired, changed); err != nil {
		return err
	}
	before, err := m.loadState(ctx)
	if err != nil {
		return err
	}
	for protocol := range changed {
		if protocol == "DNS" && !before.PlainReadable {
			return fmt.Errorf("%w: saved plain DNS configuration is not readable on this firmware", errDNSResolverReadOnly)
		}
		// The caller built desired from an earlier RCI snapshot. Its raw protocol
		// slices are deliberately left untouched while only Logical is edited.
		// Refuse to overwrite a section if Keenetic changed it between that read
		// and the mutation (for example another admin saved DNS in parallel).
		baseline := canonicalProtocolEntries(protocol, rawEntriesForProtocol(desired, protocol))
		fresh := canonicalProtocolEntries(protocol, rawEntriesForProtocol(before, protocol))
		if strings.Join(baseline, "\n") != strings.Join(fresh, "\n") {
			return fmt.Errorf("%w: %s configuration changed concurrently; refresh and retry", errDNSResolverConflict, protocol)
		}
	}
	if err := m.writeProtocols(ctx, desired, changed); err != nil {
		rollbackErr := m.restoreProtocols(ctx, before, changed)
		if rollbackErr != nil {
			return fmt.Errorf("DNS mutation failed: %v; rollback FAILED: %v", err, rollbackErr)
		}
		return fmt.Errorf("DNS mutation failed and was rolled back: %w", err)
	}
	if err := m.verifyProtocols(ctx, desired, changed); err != nil {
		rollbackErr := m.restoreProtocols(ctx, before, changed)
		if rollbackErr != nil {
			return fmt.Errorf("DNS readback mismatch: %v; rollback FAILED: %v", err, rollbackErr)
		}
		return fmt.Errorf("DNS readback mismatch; native configuration rolled back: %w", err)
	}
	if afterVerify != nil {
		if err := afterVerify(); err != nil {
			rollbackErr := m.restoreProtocols(ctx, before, changed)
			if rollbackErr != nil {
				return fmt.Errorf("metadata write failed: %v; native rollback FAILED: %v", err, rollbackErr)
			}
			return fmt.Errorf("metadata write failed; native configuration rolled back: %w", err)
		}
	}
	return nil
}

func validateDNSPhysicalLimits(desired *dnsConfigState, changed map[string]bool) error {
	if desired == nil || !changed["DoT"] {
		return nil
	}
	slots := len(desiredEntriesForProtocol(desired, "DoT"))
	if slots > dnsKeeneticDoTSlotLimit {
		return fmt.Errorf(
			"%w: Keenetic DoT limit is %d physical entries; requested %d",
			errDNSResolverConflict,
			dnsKeeneticDoTSlotLimit,
			slots,
		)
	}
	return nil
}

func (m *dnsControlManager) writeProtocols(ctx context.Context, desired *dnsConfigState, changed map[string]bool) error {
	for _, protocol := range []string{"DoT", "DoH", "DNS"} {
		if !changed[protocol] {
			continue
		}
		entries := desiredEntriesForProtocol(desired, protocol)
		if err := m.replaceProtocol(ctx, protocol, entries); err != nil {
			return err
		}
	}
	_, err := m.rci.postJSON(ctx, "/system/configuration/save", map[string]any{})
	return err
}

func (m *dnsControlManager) restoreProtocols(ctx context.Context, before *dnsConfigState, changed map[string]bool) error {
	for _, protocol := range []string{"DoT", "DoH", "DNS"} {
		if !changed[protocol] {
			continue
		}
		var entries []map[string]any
		switch protocol {
		case "DoT":
			entries = before.TLS
		case "DoH":
			entries = before.HTTPS
		case "DNS":
			entries = before.Plain
		}
		if err := m.replaceProtocol(ctx, protocol, entries); err != nil {
			return err
		}
	}
	if _, err := m.rci.postJSON(ctx, "/system/configuration/save", map[string]any{}); err != nil {
		return err
	}
	return m.verifyProtocols(ctx, before, changed)
}

func (m *dnsControlManager) replaceProtocol(ctx context.Context, protocol string, entries []map[string]any) error {
	path, ok := dnsRCIWritePath(protocol)
	if !ok {
		return fmt.Errorf("unsupported protocol %q", protocol)
	}
	if _, err := m.rci.postJSON(ctx, path, []map[string]any{{"no": true}}); err != nil {
		return fmt.Errorf("clear %s upstreams: %w", protocol, err)
	}
	if len(entries) == 0 {
		return nil
	}
	payload := make([]map[string]any, 0, len(entries))
	for _, raw := range entries {
		converted := dnsRCIPayload(protocol, raw)
		if len(converted) != 0 {
			payload = append(payload, converted)
		}
	}
	if len(payload) == 0 {
		return nil
	}
	if _, err := m.rci.postJSON(ctx, path, payload); err != nil {
		return fmt.Errorf("write %s upstreams: %w", protocol, err)
	}
	return nil
}

func (m *dnsControlManager) verifyProtocols(ctx context.Context, desired *dnsConfigState, changed map[string]bool) error {
	actual, err := m.loadState(ctx)
	if err != nil {
		return err
	}
	for _, protocol := range []string{"DoT", "DoH", "DNS"} {
		if !changed[protocol] {
			continue
		}
		want := canonicalProtocolEntries(protocol, desiredEntriesForProtocol(desired, protocol))
		got := canonicalProtocolEntries(protocol, rawEntriesForProtocol(actual, protocol))
		if strings.Join(want, "\n") != strings.Join(got, "\n") {
			return fmt.Errorf("%s readback differs: want=%v got=%v", protocol, want, got)
		}
	}
	return nil
}

func (m *dnsControlManager) loadState(ctx context.Context) (*dnsConfigState, error) {
	var proxy map[string]any
	if err := m.rci.getJSON(ctx, "/show/sc/dns-proxy", &proxy); err != nil {
		return nil, err
	}
	state := &dnsConfigState{
		TLS:     cloneMapSlice(mapSliceAt(proxy, "tls", "upstream")),
		HTTPS:   cloneMapSlice(mapSliceAt(proxy, "https", "upstream")),
		Logical: map[string]*dnsLogicalResolver{},
	}

	var savedPlain dnsSavedNameServers
	if err := m.rci.getJSON(ctx, "/show/sc/ip/name-server", &savedPlain); err == nil {
		state.Plain = cloneMapSlice(savedPlain.Server)
		state.PlainReadable = true
	} else {
		var httpErr *dnsRCIHTTPError
		if !errors.As(err, &httpErr) || (httpErr.Status != http.StatusNotFound && httpErr.Status != http.StatusBadRequest) {
			return nil, err
		}
		state.PlainReadable = false
	}

	var active struct {
		Server []dnsActiveNameServer `json:"server"`
	}
	if err := m.rci.getJSON(ctx, "/show/ip/name-server", &active); err == nil {
		state.Active = active.Server
	}

	for _, item := range logicalResolversFromRaw("DoT", state.TLS) {
		state.Logical[item.Spec.ID] = item
	}
	for _, item := range logicalResolversFromRaw("DoH", state.HTTPS) {
		state.Logical[item.Spec.ID] = item
	}
	for _, item := range logicalResolversFromRaw("DNS", state.Plain) {
		state.Logical[item.Spec.ID] = item
	}
	state.Dynamic = dynamicResolvers(state.Active, state.Logical)
	return state, nil
}

func logicalResolversFromRaw(protocol string, entries []map[string]any) []*dnsLogicalResolver {
	groups := map[string]*dnsLogicalResolver{}
	order := []string{}
	for _, raw := range entries {
		spec := specFromRaw(protocol, raw)
		if spec.Protocol == "" {
			continue
		}
		base := resolverBaseKey(spec)
		group := groups[base]
		if group == nil {
			spec.Domains = nil
			group = &dnsLogicalResolver{Spec: spec}
			groups[base] = group
			order = append(order, base)
		}
		domain := normalizeDomain(stringField(raw, "domain"))
		if domain != "" {
			group.Spec.Domains = appendUnique(group.Spec.Domains, domain)
		}
		group.RawEntries = append(group.RawEntries, cloneMap(raw))
	}
	out := make([]*dnsLogicalResolver, 0, len(order))
	for _, key := range order {
		item := groups[key]
		sort.Strings(item.Spec.Domains)
		item.Spec.ID = dnsResolverID(item.Spec)
		item.Spec.Name = friendlyDNSResolverName(item.Spec)
		item.Spec.Source = "static"
		item.Spec.PhysicalCount = len(item.RawEntries)
		out = append(out, item)
	}
	return out
}

func specFromRaw(protocol string, raw map[string]any) DNSResolverSpec {
	spec := DNSResolverSpec{Protocol: protocol}
	spec.Interface = stringField(raw, "interface")
	spec.SPKI = stringField(raw, "spki")
	spec.Format = stringField(raw, "format")
	domain := normalizeDomain(stringField(raw, "domain"))
	if domain != "" {
		spec.Domains = []string{domain}
	}
	switch protocol {
	case "DoT":
		spec.Address = strings.TrimSpace(stringField(raw, "address"))
		spec.Port = intField(raw, "port")
		if spec.Port == 0 {
			spec.Port = 853
		}
		spec.SNI = firstStringField(raw, "sni", "fqdn")
	case "DoH":
		spec.URI = firstStringField(raw, "uri", "url", "address")
	case "DNS":
		spec.Address = strings.TrimSpace(stringField(raw, "address"))
		spec.Port = intField(raw, "port")
		if spec.Port == 0 {
			spec.Port = 53
		}
	}
	return spec
}

func dynamicResolvers(active []dnsActiveNameServer, logical map[string]*dnsLogicalResolver) []DNSResolverSpec {
	staticKeys := map[string]struct{}{}
	for _, item := range logical {
		if item.Spec.Protocol == "DNS" {
			staticKeys[plainActiveKey(item.Spec.Address, firstDomain(item.Spec.Domains), item.Spec.Interface)] = struct{}{}
		}
	}
	seen := map[string]struct{}{}
	out := []DNSResolverSpec{}
	for _, server := range active {
		address := strings.TrimSpace(server.Address)
		if address == "" {
			continue
		}
		key := plainActiveKey(address, normalizeDomain(server.Domain), server.Interface)
		_, isStatic := staticKeys[key]
		if server.Service == "" && isStatic {
			continue
		}
		identity := strings.ToLower(strings.Join([]string{address, server.Domain, server.Interface, server.Service}, "|"))
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		spec := DNSResolverSpec{
			Protocol:       "DNS",
			Address:        address,
			Port:           53,
			Interface:      server.Interface,
			Dynamic:        true,
			Service:        server.Service,
			Source:         "dynamic",
			PhysicalCount:  1,
			ReadOnlyReason: "provided by Keenetic service/DHCP",
		}
		if d := normalizeDomain(server.Domain); d != "" {
			spec.Domains = []string{d}
		}
		spec.Name = friendlyDNSResolverName(spec)
		spec.ID = "dyn-" + shortHash(identity)
		out = append(out, spec)
	}
	return out
}

func desiredEntriesForProtocol(state *dnsConfigState, protocol string) []map[string]any {
	ids := make([]string, 0, len(state.Logical))
	for id, item := range state.Logical {
		if item.Spec.Protocol == protocol {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := []map[string]any{}
	for _, id := range ids {
		item := state.Logical[id]
		if len(item.RawEntries) != 0 {
			out = append(out, cloneMapSlice(item.RawEntries)...)
			continue
		}
		entries, _ := buildDNSResolverEntries(item.Spec)
		out = append(out, entries...)
	}
	return out
}

func rawEntriesForProtocol(state *dnsConfigState, protocol string) []map[string]any {
	switch protocol {
	case "DoT":
		return state.TLS
	case "DoH":
		return state.HTTPS
	case "DNS":
		return state.Plain
	default:
		return nil
	}
}

func dnsRCIWritePath(protocol string) (string, bool) {
	switch protocol {
	case "DoT":
		return "/dns-proxy/tls/upstream", true
	case "DoH":
		return "/dns-proxy/https/upstream", true
	case "DNS":
		return "/ip/name-server", true
	default:
		return "", false
	}
}

func dnsRCIPayload(protocol string, raw map[string]any) map[string]any {
	out := map[string]any{}
	switch protocol {
	case "DoT":
		copyIfPresent(out, raw, "address", "port", "domain", "interface", "spki")
		if sni := firstStringField(raw, "sni", "fqdn"); sni != "" {
			// Keenetic's saved RCI schema exposes TLS SNI as "fqdn".
			// Posting "sni" is accepted by transport but silently dropped by NDMS.
			out["fqdn"] = sni
		}
	case "DoH":
		if uri := firstStringField(raw, "uri", "url", "address"); uri != "" {
			out["uri"] = uri
		}
		copyIfPresent(out, raw, "format", "domain", "interface")
	case "DNS":
		copyIfPresent(out, raw, "address", "domain", "interface")
		if port := intField(raw, "port"); port != 0 && port != 53 {
			out["port"] = port
		}
	}
	return out
}

func buildDNSResolverEntries(spec DNSResolverSpec) ([]map[string]any, error) {
	normalized, err := normalizeDNSResolverSpec(spec)
	if err != nil {
		return nil, err
	}
	domains := normalized.Domains
	if len(domains) == 0 {
		domains = []string{""}
	}
	out := make([]map[string]any, 0, len(domains))
	for _, domain := range domains {
		entry := map[string]any{}
		switch normalized.Protocol {
		case "DoT":
			entry["address"] = normalized.Address
			if normalized.Port != 853 {
				entry["port"] = normalized.Port
			}
			if normalized.SNI != "" {
				entry["sni"] = normalized.SNI
			}
			if normalized.SPKI != "" {
				entry["spki"] = normalized.SPKI
			}
		case "DoH":
			entry["uri"] = normalized.URI
			if normalized.Format != "" {
				entry["format"] = normalized.Format
			}
		case "DNS":
			entry["address"] = normalized.Address
			if normalized.Port != 53 {
				entry["port"] = normalized.Port
			}
		}
		if normalized.Interface != "" {
			entry["interface"] = normalized.Interface
		}
		if domain != "" {
			entry["domain"] = domain
		}
		out = append(out, entry)
	}
	return out, nil
}

func normalizeDNSResolverSpec(spec DNSResolverSpec) (DNSResolverSpec, error) {
	out := spec
	out.ID = ""
	out.Dynamic = false
	out.Disabled = false
	out.Service = ""
	out.ReadOnlyReason = ""
	out.Source = "static"
	out.Protocol = strings.TrimSpace(out.Protocol)
	switch strings.ToLower(out.Protocol) {
	case "dns", "plain":
		out.Protocol = "DNS"
	case "dot", "tls":
		out.Protocol = "DoT"
	case "doh", "https":
		out.Protocol = "DoH"
	default:
		return DNSResolverSpec{}, fmt.Errorf("%w: protocol must be DNS, DoT or DoH", errDNSResolverInvalid)
	}
	out.Address = strings.TrimSpace(out.Address)
	out.URI = strings.TrimSpace(out.URI)
	out.SNI = strings.TrimSpace(out.SNI)
	out.Interface = strings.TrimSpace(out.Interface)
	out.SPKI = strings.TrimSpace(out.SPKI)
	out.Format = strings.TrimSpace(out.Format)
	out.Domains = normalizeDomains(out.Domains)
	if len(out.Domains) > 64 {
		return DNSResolverSpec{}, fmt.Errorf("%w: at most 64 domain bindings are accepted", errDNSResolverInvalid)
	}
	for _, value := range []string{out.SNI, out.Interface, out.SPKI, out.Format} {
		if !dnsSafeToken.MatchString(value) {
			return DNSResolverSpec{}, fmt.Errorf("%w: unsupported characters in resolver metadata", errDNSResolverInvalid)
		}
	}
	for _, domain := range out.Domains {
		if !dnsSafeToken.MatchString(domain) {
			return DNSResolverSpec{}, fmt.Errorf("%w: unsupported characters in domain %q", errDNSResolverInvalid, domain)
		}
	}

	switch out.Protocol {
	case "DNS":
		if net.ParseIP(strings.Trim(out.Address, "[]")) == nil {
			return DNSResolverSpec{}, fmt.Errorf("%w: plain DNS address must be an IPv4/IPv6 address", errDNSResolverInvalid)
		}
		if out.Port == 0 {
			out.Port = 53
		}
		out.URI, out.SNI, out.SPKI, out.Format = "", "", "", ""
	case "DoT":
		if out.Address == "" || strings.ContainsAny(out.Address, " \t\r\n/") {
			return DNSResolverSpec{}, fmt.Errorf("%w: DoT address is required", errDNSResolverInvalid)
		}
		if out.Port == 0 {
			out.Port = 853
		}
		out.URI, out.Format = "", ""
	case "DoH":
		parsed, err := url.Parse(out.URI)
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" {
			return DNSResolverSpec{}, fmt.Errorf("%w: DoH URI must be a valid https:// URL", errDNSResolverInvalid)
		}
		out.Address, out.SNI, out.SPKI = "", "", ""
		if out.Port == 0 {
			out.Port = 443
		}
	}
	if out.Port < 1 || out.Port > 65535 {
		return DNSResolverSpec{}, fmt.Errorf("%w: port must be between 1 and 65535", errDNSResolverInvalid)
	}
	out.Name = friendlyDNSResolverName(out)
	out.PhysicalCount = 1
	if len(out.Domains) > 0 {
		out.PhysicalCount = len(out.Domains)
	}
	return out, nil
}

func dnsResolverID(spec DNSResolverSpec) string {
	return strings.ToLower(spec.Protocol[:1]) + "-" + shortHash(resolverBaseKey(spec))
}

func resolverBaseKey(spec DNSResolverSpec) string {
	scope := "global"
	if len(spec.Domains) > 0 {
		scope = "scoped"
	}
	endpoint := strings.ToLower(strings.TrimSpace(spec.Address))
	if spec.Protocol == "DoH" {
		endpoint = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(spec.URI), "/"))
	}
	return strings.Join([]string{
		strings.ToLower(spec.Protocol), endpoint, strconv.Itoa(spec.Port), strings.ToLower(spec.SNI),
		strings.ToLower(spec.Interface), strings.ToLower(spec.SPKI), strings.ToLower(spec.Format), scope,
	}, "|")
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func friendlyDNSResolverName(spec DNSResolverSpec) string {
	host := strings.ToLower(spec.Address)
	if spec.Protocol == "DoH" {
		if parsed, err := url.Parse(spec.URI); err == nil {
			host = strings.ToLower(parsed.Hostname())
		}
	}
	if spec.SNI != "" {
		host = strings.ToLower(spec.SNI)
	}
	provider := ""
	switch {
	case strings.Contains(host, "cloudflare") || host == "1.1.1.1" || host == "1.0.0.1":
		provider = "Cloudflare"
	case strings.Contains(host, "google") || host == "8.8.8.8" || host == "8.8.4.4":
		provider = "Google"
	case strings.Contains(host, "quad9") || host == "9.9.9.9" || host == "149.112.112.112":
		provider = "Quad9"
	case strings.Contains(host, "yandex") || strings.HasPrefix(host, "77.88.8."):
		provider = "Yandex"
	}
	if provider != "" {
		if spec.Protocol == "DNS" {
			return provider + " DNS"
		}
		return provider + " " + spec.Protocol
	}
	if spec.Name != "" {
		return strings.TrimSpace(spec.Name)
	}
	if spec.Protocol == "DoH" && spec.URI != "" {
		return spec.URI
	}
	if spec.Address != "" {
		return spec.Address + " " + spec.Protocol
	}
	return spec.Protocol
}

func canonicalProtocolEntries(protocol string, entries []map[string]any) []string {
	out := make([]string, 0, len(entries))
	for _, raw := range entries {
		spec := specFromRaw(protocol, raw)
		domain := normalizeDomain(stringField(raw, "domain"))
		parts := []string{strings.ToLower(protocol)}
		switch protocol {
		case "DoT":
			parts = append(parts, strings.ToLower(spec.Address), strconv.Itoa(spec.Port), strings.ToLower(spec.SNI))
		case "DoH":
			parts = append(parts, strings.ToLower(strings.TrimSuffix(spec.URI, "/")), strings.ToLower(spec.Format))
		case "DNS":
			parts = append(parts, strings.ToLower(spec.Address), strconv.Itoa(spec.Port))
		}
		parts = append(parts, strings.ToLower(spec.Interface), strings.ToLower(spec.SPKI), domain)
		out = append(out, strings.Join(parts, "|"))
	}
	sort.Strings(out)
	return out
}

func (m *dnsControlManager) loadDisabled() (dnsDisabledStore, error) {
	store := dnsDisabledStore{Version: dnsDisabledStoreVersion}
	payload, err := os.ReadFile(m.disabledPath)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return store, err
	}
	if err := json.Unmarshal(payload, &store); err != nil {
		return store, fmt.Errorf("decode disabled DNS metadata: %w", err)
	}
	if store.Version != dnsDisabledStoreVersion {
		return store, fmt.Errorf("unsupported disabled DNS metadata version %d", store.Version)
	}
	return store, nil
}

func (m *dnsControlManager) saveDisabled(store dnsDisabledStore) error {
	store.Version = dnsDisabledStoreVersion
	if err := os.MkdirAll(filepath.Dir(m.disabledPath), 0755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp := m.disabledPath + ".tmp"
	if err := os.WriteFile(tmp, append(payload, '\n'), 0600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, m.disabledPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func disabledHasID(store dnsDisabledStore, id string) bool { return disabledIndex(store, id) >= 0 }
func disabledIndex(store dnsDisabledStore, id string) int {
	for i, record := range store.Resolvers {
		if record.Resolver.ID == id {
			return i
		}
	}
	return -1
}
func dynamicByID(items []DNSResolverSpec, id string) *DNSResolverSpec {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}
func cloneDisabledStore(in dnsDisabledStore) dnsDisabledStore {
	out := dnsDisabledStore{Version: in.Version, Resolvers: make([]dnsDisabledRecord, len(in.Resolvers))}
	for i := range in.Resolvers {
		out.Resolvers[i] = dnsDisabledRecord{Resolver: in.Resolvers[i].Resolver, RawEntries: cloneMapSlice(in.Resolvers[i].RawEntries)}
		out.Resolvers[i].Resolver.Domains = append([]string(nil), in.Resolvers[i].Resolver.Domains...)
	}
	return out
}

func mapSliceAt(root map[string]any, keys ...string) []map[string]any {
	var current any = root
	for _, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[key]
	}
	switch value := current.(type) {
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if object, ok := item.(map[string]any); ok {
				out = append(out, object)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{value}
	default:
		return nil
	}
}

func stringField(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}
func firstStringField(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringField(raw, key); value != "" {
			return value
		}
	}
	return ""
}
func intField(raw map[string]any, key string) int {
	value, ok := raw[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	default:
		return 0
	}
}
func copyIfPresent(out, raw map[string]any, keys ...string) {
	for _, key := range keys {
		if value, ok := raw[key]; ok && value != nil && fmt.Sprint(value) != "" {
			out[key] = value
		}
	}
}
func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
func cloneMapSlice(in []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, item := range in {
		out = append(out, cloneMap(item))
	}
	return out
}
func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if strings.EqualFold(existing, value) {
			return values
		}
	}
	return append(values, value)
}
func normalizeDomains(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = normalizeDomain(value)
		if value != "" {
			out = appendUnique(out, value)
		}
	}
	sort.Strings(out)
	return out
}
func normalizeDomain(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, ".")
	value = strings.TrimSuffix(value, ".")
	return value
}
func firstDomain(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
func plainActiveKey(address, domain, iface string) string {
	return strings.ToLower(strings.Join([]string{strings.TrimSpace(address), normalizeDomain(domain), strings.TrimSpace(iface)}, "|"))
}
