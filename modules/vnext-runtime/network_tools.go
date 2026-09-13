package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/probe"
)

type routeEntry struct {
	Interface   string `json:"interface"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Mask        string `json:"mask"`
	Prefix      int    `json:"prefix"`
	Metric      int    `json:"metric"`
	Flags       uint64 `json:"flags"`
}

type flowEntry struct {
	Protocol    string `json:"protocol"`
	Source      string `json:"source,omitempty"`
	Destination string `json:"destination,omitempty"`
	SourcePort  string `json:"source_port,omitempty"`
	DestPort    string `json:"destination_port,omitempty"`
	State       string `json:"state,omitempty"`
}

func registerNetworkToolsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/summary", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		routes, _ := readRoutes()
		flows, _ := readFlows(128)
		ifaces, _ := net.Interfaces()
		writeJSON(w, http.StatusOK, map[string]any{
			"interfaces": len(ifaces), "routes": len(routes), "flows_sampled": len(flows),
			"probe_engine": true, "mutation_api": false,
		})
	}))
	mux.HandleFunc("/v1/interfaces", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"interfaces": interfaceSnapshot()})
	}))
	mux.HandleFunc("/v1/routes", getOnly(func(w http.ResponseWriter, r *http.Request) {
		routes, err := readRoutes()
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": err.Error()})
			return
		}
		response := map[string]any{"routes": routes}
		if target := strings.TrimSpace(r.URL.Query().Get("target")); target != "" {
			ip, resolution := targetIP(r.Context(), target)
			response["resolution"] = resolution
			if ip != nil {
				response["selected"] = selectRoute(routes, ip)
			}
		}
		writeJSON(w, http.StatusOK, response)
	}))
	mux.HandleFunc("/v1/flows", getOnly(func(w http.ResponseWriter, _ *http.Request) {
		flows, source := readFlows(256)
		writeJSON(w, http.StatusOK, map[string]any{"flows": flows, "source": source, "limit": 256})
	}))
	mux.HandleFunc("/v1/doctor", getOnly(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, networkDoctor(r))
	}))
}

func interfaceSnapshot() []map[string]any {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(interfaces))
	for _, iface := range interfaces {
		addresses, _ := iface.Addrs()
		addrStrings := make([]string, 0, len(addresses))
		for _, addr := range addresses {
			addrStrings = append(addrStrings, addr.String())
			if len(addrStrings) >= 16 {
				break
			}
		}
		out = append(out, map[string]any{
			"name": iface.Name, "index": iface.Index, "mtu": iface.MTU,
			"hardware_addr": iface.HardwareAddr.String(), "flags": iface.Flags.String(), "addresses": addrStrings,
		})
	}
	return out
}

func readRoutes() ([]routeEntry, error) {
	file, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	out := make([]routeEntry, 0, 32)
	scanner := bufio.NewScanner(file)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || len(out) >= 256 {
			continue
		}
		dst := routeHexIP(fields[1])
		gw := routeHexIP(fields[2])
		mask := routeHexIP(fields[7])
		if dst == nil || gw == nil || mask == nil {
			continue
		}
		metric, _ := strconv.Atoi(fields[6])
		flags, _ := strconv.ParseUint(fields[3], 16, 64)
		mask4 := net.IPMask(mask.To4())
		ones, _ := mask4.Size()
		out = append(out, routeEntry{
			Interface: fields[0], Destination: dst.String(), Gateway: gw.String(), Mask: mask.String(), Prefix: ones, Metric: metric, Flags: flags,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Prefix != out[j].Prefix {
			return out[i].Prefix > out[j].Prefix
		}
		return out[i].Metric < out[j].Metric
	})
	return out, nil
}

func routeHexIP(raw string) net.IP {
	if len(raw) != 8 {
		return nil
	}
	bytes, err := hex.DecodeString(raw)
	if err != nil || len(bytes) != 4 {
		return nil
	}
	return net.IPv4(bytes[3], bytes[2], bytes[1], bytes[0])
}

func targetIP(ctx context.Context, target string) (net.IP, probe.Result) {
	result := probe.Resolve(ctx, target, 2*time.Second)
	if !result.OK || len(result.Addresses) == 0 {
		return nil, result
	}
	for _, raw := range result.Addresses {
		if ip := net.ParseIP(raw); ip != nil && ip.To4() != nil {
			return ip.To4(), result
		}
	}
	return nil, result
}

func selectRoute(routes []routeEntry, ip net.IP) *routeEntry {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	for i := range routes {
		route := &routes[i]
		dst := net.ParseIP(route.Destination).To4()
		maskIP := net.ParseIP(route.Mask).To4()
		if dst == nil || maskIP == nil {
			continue
		}
		mask := net.IPMask(maskIP)
		if ip4.Mask(mask).Equal(dst.Mask(mask)) {
			copy := *route
			return &copy
		}
	}
	return nil
}

func readFlows(limit int) ([]flowEntry, string) {
	paths := []string{"/proc/net/nf_conntrack", "/proc/net/ip_conntrack"}
	if limit <= 0 || limit > 512 {
		limit = 256
	}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		defer file.Close()
		out := make([]flowEntry, 0, minInt(limit, 64))
		scanner := bufio.NewScanner(file)
		buffer := make([]byte, 4096)
		scanner.Buffer(buffer, 64<<10)
		for scanner.Scan() && len(out) < limit {
			if flow, ok := parseFlow(scanner.Text()); ok {
				out = append(out, flow)
			}
		}
		return out, path
	}
	return []flowEntry{}, "unavailable"
}

func parseFlow(line string) (flowEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return flowEntry{}, false
	}
	flow := flowEntry{}
	for _, field := range fields {
		if flow.Protocol == "" && (field == "tcp" || field == "udp" || field == "icmp" || field == "icmpv6") {
			flow.Protocol = field
			continue
		}
		if flow.State == "" && (field == "ESTABLISHED" || field == "TIME_WAIT" || field == "SYN_SENT" || field == "CLOSE" || field == "UNREPLIED") {
			flow.State = field
		}
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			continue
		}
		switch key {
		case "src":
			if flow.Source == "" {
				flow.Source = value
			}
		case "dst":
			if flow.Destination == "" {
				flow.Destination = value
			}
		case "sport":
			if flow.SourcePort == "" {
				flow.SourcePort = value
			}
		case "dport":
			if flow.DestPort == "" {
				flow.DestPort = value
			}
		}
	}
	return flow, flow.Protocol != "" && (flow.Source != "" || flow.Destination != "")
}

func networkDoctor(r *http.Request) map[string]any {
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target == "" {
		target = "1.1.1.1"
	}
	if len(target) > 253 || strings.ContainsAny(target, "/\\\x00") {
		return map[string]any{"ok": false, "first_failure": "target-validation", "error": "invalid target"}
	}
	port := 443
	if raw := strings.TrimSpace(r.URL.Query().Get("port")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 65535 {
			return map[string]any{"ok": false, "first_failure": "port-validation", "error": "invalid port"}
		}
		port = parsed
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resolution := probe.Resolve(ctx, target, 2*time.Second)
	result := map[string]any{"target": target, "port": port, "resolution": resolution, "mutation_api": false}
	if !resolution.OK || len(resolution.Addresses) == 0 {
		result["ok"] = false
		result["first_failure"] = "dns"
		return result
	}

	routes, _ := readRoutes()
	var selected *routeEntry
	var selectedIP string
	for _, raw := range resolution.Addresses {
		ip := net.ParseIP(raw)
		if ip != nil && ip.To4() != nil {
			selected = selectRoute(routes, ip)
			selectedIP = ip.String()
			break
		}
	}
	result["route"] = selected
	if selected == nil && net.ParseIP(selectedIP) != nil && net.ParseIP(selectedIP).To4() != nil {
		result["ok"] = false
		result["first_failure"] = "route"
		return result
	}

	address := net.JoinHostPort(resolution.Addresses[0], strconv.Itoa(port))
	tcp := probe.TCP(ctx, address, 2500*time.Millisecond)
	result["transport"] = tcp
	if !tcp.OK {
		result["ok"] = false
		result["first_failure"] = "transport"
		return result
	}
	result["ok"] = true
	result["first_failure"] = ""
	return result
}
