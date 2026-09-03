package main

import (
	"bufio"
	"encoding/hex"
	"net"
	"net/http"
	"os"
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
	Name      string        `json:"name"`
	Index     int           `json:"index"`
	MTU       int           `json:"mtu"`
	MAC       string        `json:"mac,omitempty"`
	Addresses []string      `json:"addresses,omitempty"`
	OperState string        `json:"oper_state,omitempty"`
	SpeedMbps int           `json:"speed_mbps,omitempty"`
	Duplex    string        `json:"duplex,omitempty"`
	RXBytes   uint64        `json:"rx_bytes"`
	RXPackets uint64        `json:"rx_packets"`
	RXErrors  uint64        `json:"rx_errors"`
	RXDrops   uint64        `json:"rx_drops"`
	TXBytes   uint64        `json:"tx_bytes"`
	TXPackets uint64        `json:"tx_packets"`
	TXErrors  uint64        `json:"tx_errors"`
	TXDrops   uint64        `json:"tx_drops"`
	Rate      netRate       `json:"rate"`
	Wireless  *wirelessStat `json:"wireless,omitempty"`
}

type routeInfo struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Mask        string `json:"mask"`
	Metric      int    `json:"metric"`
	Flags       string `json:"flags"`
}

type networkCollector struct {
	mu       sync.RWMutex
	interval time.Duration
	previous map[string]netCounters
	rates    map[string]netRate
	sampled  time.Time
	stop     chan struct{}
	done     chan struct{}
}

func newNetworkCollector(interval time.Duration) *networkCollector {
	if interval < time.Second {
		interval = 2 * time.Second
	}
	n := &networkCollector{
		interval: interval,
		previous: readNetCounters(),
		rates:    map[string]netRate{},
		sampled:  time.Now(),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
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

func (s *moduleServer) registerNetwork(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		rates, sampled := s.network.snapshot()
		interfaces := readInterfaces(rates)

		var rxBPS, txBPS float64
		var errors, drops uint64
		for _, item := range interfaces {
			if item.Name == "lo" {
				continue
			}
			rxBPS += item.Rate.RXBPS
			txBPS += item.Rate.TXBPS
			errors += item.RXErrors + item.TXErrors
			drops += item.RXDrops + item.TXDrops
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"interface_count": len(interfaces),
			"rx_bps":          rxBPS,
			"tx_bps":          txBPS,
			"errors":          errors,
			"drops":           drops,
			"conntrack_count": readIntFile("/proc/sys/net/netfilter/nf_conntrack_count"),
			"conntrack_max":   readIntFile("/proc/sys/net/netfilter/nf_conntrack_max"),
			"sampled_at":      sampled,
			"sample_seconds":  int(s.network.interval.Seconds()),
		})
	}))

	mux.HandleFunc("/v1/interfaces", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		rates, sampled := s.network.snapshot()
		writeJSON(w, http.StatusOK, map[string]any{
			"interfaces": readInterfaces(rates),
			"sampled_at": sampled,
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
			Name: iface.Name, Index: iface.Index, MTU: iface.MTU,
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

	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == "lo" {
			return false
		}
		if out[j].Name == "lo" {
			return true
		}
		if out[i].OperState != out[j].OperState {
			return out[i].OperState == "up"
		}
		return out[i].Name < out[j].Name
	})
	return out
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
