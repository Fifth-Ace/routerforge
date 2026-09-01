package main

import (
	"bufio"
	"net"
	"os"
	"sort"
	"strings"
	"time"
)

func firstValue(lines []string, key string) string {
	prefix := key + ":"
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func firstValueBefore(lines []string, key, stop string) string {
	prefix := key + ":"
	stopLine := stop + ":"
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == stopLine {
			break
		}
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func sectionValue(lines []string, section, key string) string {
	start := -1
	for i, raw := range lines {
		if strings.TrimSpace(raw) == section+":" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	prefix := key + ":"
	for _, raw := range lines[start:] {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(raw, " ") && !strings.HasPrefix(raw, "\t") {
			break
		}
		if strings.HasSuffix(line, ":") && !strings.HasPrefix(line, prefix) {
			// nested sections are fine; only stop once another top-level marker is seen below.
		}
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func hasSection(lines []string, section string) bool {
	needle := section + ":"
	for _, raw := range lines {
		if strings.TrimSpace(raw) == needle {
			return true
		}
	}
	return false
}

func interfaceValues(lines []string) (id, name string) {
	start := -1
	for i, raw := range lines {
		if strings.TrimSpace(raw) == "interface:" {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", ""
	}
	for _, raw := range lines[start:] {
		line := strings.TrimSpace(raw)
		if line == "dhcp:" || line == "registered:" || line == "traffic-shape:" || line == "ssdp:" {
			break
		}
		if id == "" && strings.HasPrefix(line, "id:") {
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		}
		if name == "" && strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		}
	}
	return
}

func normalizeClientPolicy(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "System"
	}
	return p
}

func clientAccess(network, ssid, ap, port string, mesh bool) string {
	if ssid != "" || ap != "" {
		prefix := "Wi-Fi"
		if mesh {
			prefix += " · Mesh"
		}
		if ssid != "" {
			return prefix + " · " + ssid
		}
		return prefix + " · " + ap
	}
	if port != "" {
		if mesh {
			return "Ethernet · Mesh · port " + port
		}
		return "Ethernet · port " + port
	}
	if network != "" {
		return "Network · " + network
	}
	return "Unknown"
}

func parseHotspot(text string) []ClientInfo {
	var blocks [][]string
	var cur []string
	s := bufio.NewScanner(strings.NewReader(text))
	for s.Scan() {
		raw := s.Text()
		if strings.TrimSpace(raw) == "host:" {
			if len(cur) > 0 {
				blocks = append(blocks, cur)
			}
			cur = []string{raw}
			continue
		}
		if cur != nil {
			cur = append(cur, raw)
		}
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}
	out := make([]ClientInfo, 0, len(blocks))
	for _, b := range blocks {
		ip := firstValueBefore(b, "ip", "interface")
		if net.ParseIP(ip) == nil {
			continue
		}
		networkID, network := interfaceValues(b)
		ssid := firstValue(b, "ssid")
		ap := firstValue(b, "ap")
		port := firstValue(b, "port")
		mesh := hasSection(b, "mws")
		meshCID := sectionValue(b, "mws", "cid")
		active := strings.EqualFold(firstValue(b, "active"), "yes")
		c := ClientInfo{
			IP:        ip,
			MAC:       strings.ToLower(firstValueBefore(b, "mac", "interface")),
			Hostname:  firstValueBefore(b, "hostname", "interface"),
			Name:      firstValueBefore(b, "name", "interface"),
			Policy:    normalizeClientPolicy(firstValue(b, "policy")),
			Network:   network,
			NetworkID: networkID,
			SSID:      ssid,
			AP:        ap,
			Port:      port,
			Mesh:      mesh,
			MeshCID:   meshCID,
			Active:    active,
		}
		c.Access = clientAccess(c.Network, c.SSID, c.AP, c.Port, c.Mesh)
		if c.Name == "" {
			c.Name = c.Hostname
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out
}

func readARP() map[string]string {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil
	}
	defer f.Close()
	out := make(map[string]string)
	s := bufio.NewScanner(f)
	first := true
	for s.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(s.Text())
		if len(fields) < 4 || net.ParseIP(fields[0]) == nil {
			continue
		}
		mac := strings.ToLower(fields[3])
		if mac == "00:00:00:00:00:00" {
			continue
		}
		out[fields[0]] = mac
	}
	return out
}

func discoverClients() ([]ClientInfo, error) {
	text, err := ndmcOutput("show ip hotspot", 10*time.Second)
	if err != nil {
		return nil, err
	}
	clients := parseHotspot(text)
	arp := readARP()
	seen := make(map[string]bool, len(clients))
	for i := range clients {
		seen[clients[i].IP] = true
		if clients[i].MAC == "" {
			clients[i].MAC = arp[clients[i].IP]
		}
	}
	for ip, mac := range arp {
		if seen[ip] || !isPrivateOrLocalIP(net.ParseIP(ip)) {
			continue
		}
		clients = append(clients, ClientInfo{IP: ip, MAC: mac, Policy: "System", Access: "Unknown", Active: true})
	}
	return clients, nil
}

func clientRegistryLoop(store *Store, log *EventLogger) {
	refresh := func() {
		clients, err := discoverClients()
		if err != nil {
			store.SetClientRegistryError(err.Error())
			return
		}
		store.UpdateClientRegistry(clients)
	}
	refresh()
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		refresh()
	}
}
