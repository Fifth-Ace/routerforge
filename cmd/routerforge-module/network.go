package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type netCounters struct {
	RXBytes   uint64
	RXPackets uint64
	RXErrors  uint64
	RXDrops   uint64
	TXBytes   uint64
	TXPackets uint64
	TXErrors  uint64
	TXDrops   uint64
}

type netRate struct {
	RXBPS float64 `json:"rx_bps"`
	TXBPS float64 `json:"tx_bps"`
	RXPPS float64 `json:"rx_pps"`
	TXPPS float64 `json:"tx_pps"`
}

type wirelessStat struct {
	Quality float64 `json:"quality,omitempty"`
	Signal  float64 `json:"signal_dbm,omitempty"`
	Noise   float64 `json:"noise_dbm,omitempty"`
}

type interfaceInfo struct {
	Name                string        `json:"name"`
	DisplayName         string        `json:"display_name,omitempty"`
	PrimaryAddress      string        `json:"primary_address,omitempty"`
	KeeneticID          string        `json:"keenetic_id,omitempty"`
	KeeneticName        string        `json:"keenetic_name,omitempty"`
	KeeneticDescription string        `json:"keenetic_description,omitempty"`
	KeeneticType        string        `json:"keenetic_type,omitempty"`
	LogicalState        string        `json:"logical_state,omitempty"`
	Connected           bool          `json:"connected,omitempty"`
	Index               int           `json:"index"`
	MTU                 int           `json:"mtu"`
	MAC                 string        `json:"mac,omitempty"`
	Addresses           []string      `json:"addresses,omitempty"`
	OperState           string        `json:"oper_state,omitempty"`
	SpeedMbps           int           `json:"speed_mbps,omitempty"`
	Duplex              string        `json:"duplex,omitempty"`
	RXBytes             uint64        `json:"rx_bytes"`
	RXPackets           uint64        `json:"rx_packets"`
	RXErrors            uint64        `json:"rx_errors"`
	RXDrops             uint64        `json:"rx_drops"`
	TXBytes             uint64        `json:"tx_bytes"`
	TXPackets           uint64        `json:"tx_packets"`
	TXErrors            uint64        `json:"tx_errors"`
	TXDrops             uint64        `json:"tx_drops"`
	Rate                netRate       `json:"rate"`
	Wireless            *wirelessStat `json:"wireless,omitempty"`
}

type routeInfo struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Mask        string `json:"mask"`
	Metric      int    `json:"metric"`
	Flags       string `json:"flags"`
}

type keeneticInterface struct {
	ID            string
	InterfaceName string
	Description   string
	Type          string
	Link          string
	State         string
	Address       string
	Connected     bool
	SystemName    string
}

type networkCollector struct {
	mu       sync.RWMutex
	interval time.Duration
	previous map[string]netCounters
	rates    map[string]netRate
	sampled  time.Time
	stop     chan struct{}
	done     chan struct{}

	keeneticMu      sync.Mutex
	keenetic        []keeneticInterface
	keeneticScanned time.Time
	keeneticTTL     time.Duration
	systemNames     map[string]string
}

func newNetworkCollector(interval time.Duration) *networkCollector {
	if interval < time.Second {
		interval = 2 * time.Second
	}
	n := &networkCollector{
		interval:    interval,
		previous:    readNetCounters(),
		rates:       map[string]netRate{},
		sampled:     time.Now(),
		stop:        make(chan struct{}),
		done:        make(chan struct{}),
		keeneticTTL: 5 * time.Second,
		systemNames: map[string]string{},
	}
	go n.run()
	return n
}

func (n *networkCollector) Close() {
	select {
	case <-n.stop:
		return
	default:
		close(n.stop)
	}
	<-n.done
}

func (n *networkCollector) run() {
	defer close(n.done)
	ticker := time.NewTicker(n.interval)
	defer ticker.Stop()

	for {
		select {
		case <-n.stop:
			return
		case <-ticker.C:
			n.sample()
		}
	}
}

func (n *networkCollector) sample() {
	now := time.Now()
	current := readNetCounters()

	n.mu.Lock()
	defer n.mu.Unlock()

	elapsed := now.Sub(n.sampled).Seconds()
	if elapsed <= 0 {
		elapsed = n.interval.Seconds()
	}
	rates := make(map[string]netRate, len(current))
	for name, cur := range current {
		prev, ok := n.previous[name]
		if !ok {
			continue
		}
		rates[name] = netRate{
			RXBPS: counterDelta(cur.RXBytes, prev.RXBytes) / elapsed,
			TXBPS: counterDelta(cur.TXBytes, prev.TXBytes) / elapsed,
			RXPPS: counterDelta(cur.RXPackets, prev.RXPackets) / elapsed,
			TXPPS: counterDelta(cur.TXPackets, prev.TXPackets) / elapsed,
		}
	}
	n.previous = current
	n.rates = rates
	n.sampled = now
}

func (n *networkCollector) snapshot() (map[string]netRate, time.Time) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	out := make(map[string]netRate, len(n.rates))
	for key, value := range n.rates {
		out[key] = value
	}
	return out, n.sampled
}

func (n *networkCollector) keeneticSnapshot() []keeneticInterface {
	n.keeneticMu.Lock()
	defer n.keeneticMu.Unlock()

	if len(n.keenetic) > 0 && time.Since(n.keeneticScanned) < n.keeneticTTL {
		return append([]keeneticInterface(nil), n.keenetic...)
	}

	items, err := readKeeneticInterfaceState()
	if err != nil {
		if !n.keeneticScanned.IsZero() {
			n.keeneticScanned = time.Now()
		}
		return append([]keeneticInterface(nil), n.keenetic...)
	}

	visible := make([]keeneticInterface, 0, len(items))
	for _, item := range items {
		if !shouldExposeKeeneticInterface(item) {
			continue
		}
		systemName := n.systemNames[item.ID]
		if systemName == "" {
			systemName = readKeeneticSystemName(item.ID)
			if systemName != "" {
				n.systemNames[item.ID] = systemName
			}
		}
		item.SystemName = systemName
		if item.SystemName == "" {
			continue
		}
		visible = append(visible, item)
	}

	n.keenetic = visible
	n.keeneticScanned = time.Now()
	return append([]keeneticInterface(nil), visible...)
}

func (s *moduleServer) registerNetwork(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		rates, sampled := s.network.snapshot()
		systemInterfaces := readInterfaces(rates)
		interfaces := userFacingInterfaces(systemInterfaces, s.network.keeneticSnapshot())

		var rxBPS, txBPS float64
		var errors, drops uint64
		for _, item := range interfaces {
			rxBPS += item.Rate.RXBPS
			txBPS += item.Rate.TXBPS
			errors += item.RXErrors + item.TXErrors
			drops += item.RXDrops + item.TXDrops
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"interface_count":        len(interfaces),
			"system_interface_count": len(systemInterfaces),
			"rx_bps":                 rxBPS,
			"tx_bps":                 txBPS,
			"errors":                 errors,
			"drops":                  drops,
			"conntrack_count":        readIntFile("/proc/sys/net/netfilter/nf_conntrack_count"),
			"conntrack_max":          readIntFile("/proc/sys/net/netfilter/nf_conntrack_max"),
			"sampled_at":             sampled,
			"sample_seconds":         int(s.network.interval.Seconds()),
		})
	}))

	mux.HandleFunc("/v1/interfaces", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		rates, sampled := s.network.snapshot()
		keenetic := s.network.keeneticSnapshot()
		systemInterfaces := enrichSystemInterfaces(readInterfaces(rates), keenetic)
		writeJSON(w, http.StatusOK, map[string]any{
			"interfaces":        userFacingInterfaces(systemInterfaces, keenetic),
			"system_interfaces": systemInterfaces,
			"sampled_at":        sampled,
		})
	}))

	mux.HandleFunc("/v1/routes", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"routes": readIPv4Routes()})
	}))
}

func readNetCounters() map[string]netCounters {
	out := map[string]netCounters{}
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		u := func(i int) uint64 {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			return v
		}
		out[name] = netCounters{
			RXBytes: u(0), RXPackets: u(1), RXErrors: u(2), RXDrops: u(3),
			TXBytes: u(8), TXPackets: u(9), TXErrors: u(10), TXDrops: u(11),
		}
	}
	return out
}

func readInterfaces(rates map[string]netRate) []interfaceInfo {
	counters := readNetCounters()
	wireless := readWirelessStats()
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	out := make([]interfaceInfo, 0, len(ifaces))
	for _, iface := range ifaces {
		addresses, _ := iface.Addrs()
		addrStrings := make([]string, 0, len(addresses))
		for _, address := range addresses {
			addrStrings = append(addrStrings, address.String())
		}
		sort.Strings(addrStrings)

		counter := counters[iface.Name]
		item := interfaceInfo{
			Name: iface.Name, DisplayName: iface.Name, PrimaryAddress: bestInterfaceAddress(addrStrings),
			Index: iface.Index, MTU: iface.MTU,
			MAC: iface.HardwareAddr.String(), Addresses: addrStrings,
			OperState: readTrimmed(filepath.Join("/sys/class/net", iface.Name, "operstate")),
			SpeedMbps: readIntFile(filepath.Join("/sys/class/net", iface.Name, "speed")),
			Duplex:    strings.ToLower(readTrimmed(filepath.Join("/sys/class/net", iface.Name, "duplex"))),
			RXBytes:   counter.RXBytes, RXPackets: counter.RXPackets, RXErrors: counter.RXErrors, RXDrops: counter.RXDrops,
			TXBytes: counter.TXBytes, TXPackets: counter.TXPackets, TXErrors: counter.TXErrors, TXDrops: counter.TXDrops,
			Rate: rates[iface.Name],
		}
		if stat, ok := wireless[iface.Name]; ok {
			copyStat := stat
			item.Wireless = &copyStat
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func enrichSystemInterfaces(system []interfaceInfo, keenetic []keeneticInterface) []interfaceInfo {
	meta := make(map[string]keeneticInterface, len(keenetic))
	for _, item := range keenetic {
		if item.SystemName != "" {
			meta[item.SystemName] = item
		}
	}
	out := append([]interfaceInfo(nil), system...)
	for i := range out {
		if item, ok := meta[out[i].Name]; ok {
			applyKeeneticMetadata(&out[i], item)
		}
	}
	return out
}

func userFacingInterfaces(system []interfaceInfo, keenetic []keeneticInterface) []interfaceInfo {
	byName := make(map[string]interfaceInfo, len(system))
	for _, item := range system {
		byName[item.Name] = item
	}

	out := make([]interfaceInfo, 0, len(keenetic))
	for _, logical := range keenetic {
		item, ok := byName[logical.SystemName]
		if !ok {
			continue
		}
		applyKeeneticMetadata(&item, logical)
		out = append(out, item)
	}

	if len(out) == 0 {
		return fallbackActiveInterfaces(system)
	}

	sort.SliceStable(out, func(i, j int) bool {
		left := strings.ToLower(firstNonEmpty(out[i].DisplayName, out[i].Name))
		right := strings.ToLower(firstNonEmpty(out[j].DisplayName, out[j].Name))
		if left != right {
			return left < right
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func applyKeeneticMetadata(item *interfaceInfo, logical keeneticInterface) {
	item.DisplayName = preferredKeeneticDisplayName(logical)
	item.KeeneticID = logical.ID
	item.KeeneticName = logical.InterfaceName
	item.KeeneticDescription = logical.Description
	item.KeeneticType = logical.Type
	item.LogicalState = logical.State
	item.Connected = logical.Connected
	if strings.TrimSpace(logical.Address) != "" {
		item.PrimaryAddress = strings.TrimSpace(logical.Address)
	}
}

func preferredKeeneticDisplayName(item keeneticInterface) string {
	if value := strings.TrimSpace(item.Description); value != "" {
		return value
	}
	if value := strings.TrimSpace(item.InterfaceName); value != "" && !numericOnly(value) {
		return value
	}
	return firstNonEmpty(strings.TrimSpace(item.ID), strings.TrimSpace(item.SystemName))
}

func numericOnly(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fallbackActiveInterfaces(system []interfaceInfo) []interfaceInfo {
	var out []interfaceInfo
	for _, item := range system {
		if item.Name == "lo" || fallbackSystemPlaceholder(item.Name) {
			continue
		}
		if item.OperState != "up" && item.PrimaryAddress == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func fallbackSystemPlaceholder(name string) bool {
	lower := strings.ToLower(name)
	for _, exact := range []string{"dummy0", "gre0", "gretap0", "ip6tnl0", "sit0", "tunl0", "ethoip0", "ezcfg0"} {
		if lower == exact {
			return true
		}
	}
	return false
}

func bestInterfaceAddress(addresses []string) string {
	for _, value := range addresses {
		host := value
		if slash := strings.IndexByte(host, '/'); slash >= 0 {
			host = host[:slash]
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		if ip.To4() != nil {
			return host
		}
	}
	for _, value := range addresses {
		host := value
		if slash := strings.IndexByte(host, '/'); slash >= 0 {
			host = host[:slash]
		}
		ip := net.ParseIP(host)
		if ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return host
		}
	}
	return ""
}

func readKeeneticInterfaceState() ([]keeneticInterface, error) {
	if _, err := exec.LookPath("ndmc"); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ndmc", "-c", "show interface").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return parseKeeneticInterfaces(string(output)), nil
}

func parseKeeneticInterfaces(output string) []keeneticInterface {
	var parsed []keeneticInterface
	var current *keeneticInterface

	flush := func() {
		if current == nil || strings.TrimSpace(current.ID) == "" {
			return
		}
		parsed = append(parsed, *current)
	}

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "id" {
			flush()
			current = &keeneticInterface{ID: value}
			continue
		}
		if current == nil {
			continue
		}
		switch key {
		case "interface-name":
			current.InterfaceName = value
		case "description":
			current.Description = value
		case "type":
			current.Type = value
		case "link":
			current.Link = strings.ToLower(value)
		case "connected":
			current.Connected = strings.EqualFold(value, "yes") || strings.EqualFold(value, "true")
		case "state":
			current.State = strings.ToLower(value)
		case "address":
			current.Address = value
		}
	}
	flush()

	best := map[string]keeneticInterface{}
	order := []string{}
	for _, item := range parsed {
		previous, exists := best[item.ID]
		if !exists {
			best[item.ID] = item
			order = append(order, item.ID)
			continue
		}
		if keeneticInterfaceScore(item) > keeneticInterfaceScore(previous) {
			best[item.ID] = item
		}
	}
	out := make([]keeneticInterface, 0, len(best))
	for _, id := range order {
		out = append(out, best[id])
	}
	return out
}

func keeneticInterfaceScore(item keeneticInterface) int {
	score := 0
	for _, value := range []string{item.InterfaceName, item.Description, item.Type, item.Link, item.State, item.Address} {
		if strings.TrimSpace(value) != "" {
			score++
		}
	}
	if item.Connected {
		score++
	}
	return score
}

func shouldExposeKeeneticInterface(item keeneticInterface) bool {
	if !item.Connected || strings.ToLower(strings.TrimSpace(item.State)) != "up" {
		return false
	}
	typeName := strings.ToLower(strings.TrimSpace(item.Type))
	switch typeName {
	case "port", "wifimaster", "vlan":
		return false
	}
	if strings.TrimSpace(item.Address) != "" {
		return true
	}
	switch typeName {
	case "accesspoint", "openvpn", "wireguard", "opkgtun", "proxy", "pptp", "pppoe", "l2tp", "sstp", "ipsec":
		return true
	}
	return strings.TrimSpace(item.Description) != "" && strings.TrimSpace(item.InterfaceName) != strings.TrimSpace(item.ID)
}

func readKeeneticSystemName(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	for _, r := range id {
		if !(r == '/' || r == '-' || r == '_' || r == '.' || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')) {
			return ""
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "ndmc", "-c", "show interface "+id+" system-name").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "system-name:") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "system-name:"))
	}
	return ""
}

func readWirelessStats() map[string]wirelessStat {
	out := map[string]wirelessStat{}
	f, err := os.Open("/proc/net/wireless")
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		fields := strings.Fields(parts[1])
		if len(fields) < 4 {
			continue
		}
		parse := func(value string) float64 {
			n, _ := strconv.ParseFloat(strings.TrimSuffix(value, "."), 64)
			return n
		}
		out[strings.TrimSpace(parts[0])] = wirelessStat{
			Quality: parse(fields[1]), Signal: parse(fields[2]), Noise: parse(fields[3]),
		}
	}
	return out
}

func readIPv4Routes() []routeInfo {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil
	}
	defer f.Close()

	first := true
	var out []routeInfo
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(sc.Text())
		if len(fields) < 11 {
			continue
		}
		destination, ok1 := decodeIPv4RouteHex(fields[1])
		gateway, ok2 := decodeIPv4RouteHex(fields[2])
		mask, ok3 := decodeIPv4RouteHex(fields[7])
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		metric, _ := strconv.Atoi(fields[6])
		out = append(out, routeInfo{
			Interface: fields[0], Destination: destination,
			Gateway: gateway, Mask: mask, Metric: metric, Flags: fields[3],
		})
	}

	sort.Slice(out, func(i, j int) bool {
		iDefault := out[i].Destination == "0.0.0.0"
		jDefault := out[j].Destination == "0.0.0.0"
		if iDefault != jDefault {
			return iDefault
		}
		if out[i].Metric != out[j].Metric {
			return out[i].Metric < out[j].Metric
		}
		if out[i].Destination != out[j].Destination {
			return out[i].Destination < out[j].Destination
		}
		return out[i].Interface < out[j].Interface
	})
	return out
}

func decodeIPv4RouteHex(value string) (string, bool) {
	if len(value) != 8 {
		return "", false
	}
	b, err := hex.DecodeString(value)
	if err != nil || len(b) != 4 {
		return "", false
	}
	return net.IPv4(b[3], b[2], b[1], b[0]).String(), true
}

func readIntFile(path string) int {
	value, _ := strconv.Atoi(strings.TrimSpace(readTrimmed(path)))
	return value
}
