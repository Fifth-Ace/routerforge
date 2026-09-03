package main

import (
	"bufio"
	"context"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type policyRoute struct {
	Name        string
	Description string
	Mark        uint32
	Table       int
	HasDefault  bool
}

func ndmcOutput(command string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ndmc", "-c", command).Output()
	return string(out), err
}

func parseIPPolicies(text string) map[string]policyRoute {
	out := make(map[string]policyRoute)
	var cur *policyRoute
	s := bufio.NewScanner(strings.NewReader(text))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "policy, name =") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "policy, name ="))
			name := rest
			if i := strings.IndexAny(name, ",:"); i >= 0 {
				name = name[:i]
			}
			name = strings.Trim(strings.TrimSpace(name), `"`)
			if name == "" {
				cur = nil
				continue
			}
			description := ""
			if i := strings.Index(rest, "description ="); i >= 0 {
				description = strings.TrimSpace(rest[i+len("description ="):])
				description = strings.TrimSuffix(description, ":")
				description = strings.Trim(strings.TrimSpace(description), `"`)
			}
			p := policyRoute{Name: name, Description: description}
			out[name] = p
			cur = &p
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "mark:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "mark:"))
			raw = strings.TrimPrefix(strings.ToLower(raw), "0x")
			if v, err := strconv.ParseUint(raw, 16, 32); err == nil {
				cur.Mark = uint32(v)
				p := out[cur.Name]
				p.Mark = cur.Mark
				out[cur.Name] = p
			}
			continue
		}
		if strings.HasPrefix(line, "destination:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "destination:"))
			if raw == "0.0.0.0/0" {
				cur.HasDefault = true
				p := out[cur.Name]
				p.HasDefault = true
				out[cur.Name] = p
			}
			continue
		}
		if strings.HasPrefix(line, "table4:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "table4:"))
			if v, err := strconv.Atoi(raw); err == nil {
				cur.Table = v
				p := out[cur.Name]
				p.Table = v
				out[cur.Name] = p
			}
		}
	}
	return out
}

func discoverPolicyRoutes() map[string]policyRoute {
	text, err := ndmcOutput("show ip policy", 6*time.Second)
	if err != nil {
		return nil
	}
	return parseIPPolicies(text)
}

// parseKeeneticInterfaceIPs extracts the stable Keenetic interface id and its
// IPv4 address. Matching by IP is much more reliable than guessing ethX names.
func parseKeeneticInterfaceIPs(text string) map[string]string {
	out := make(map[string]string)
	var id string
	s := bufio.NewScanner(strings.NewReader(text))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if strings.HasPrefix(line, "Interface, name =") {
			id = ""
			continue
		}
		if strings.HasPrefix(line, "id:") && id == "" {
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			continue
		}
		if strings.HasPrefix(line, "address:") && id != "" {
			ip := strings.TrimSpace(strings.TrimPrefix(line, "address:"))
			if net.ParseIP(ip) != nil {
				out[id] = ip
			}
		}
	}
	return out
}

func localInterfaceByIP() map[string]string {
	out := make(map[string]string)
	ifs, _ := net.Interfaces()
	for _, iface := range ifs {
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			host := a.String()
			if h, _, err := net.ParseCIDR(host); err == nil && h != nil {
				out[h.String()] = iface.Name
				continue
			}
			if i := strings.IndexByte(host, '/'); i >= 0 {
				host = host[:i]
			}
			if net.ParseIP(host) != nil {
				out[host] = iface.Name
			}
		}
	}
	return out
}

func keeneticLinuxInterfacesFromText(text string) map[string]string {
	kip := parseKeeneticInterfaceIPs(text)
	lip := localInterfaceByIP()
	out := make(map[string]string)
	for kid, ip := range kip {
		if linux := lip[ip]; linux != "" {
			out[kid] = linux
		}
	}
	return out
}

func discoverKeeneticLinuxInterfaces() map[string]string {
	text, err := ndmcOutput("show interface", 8*time.Second)
	if err != nil {
		return nil
	}
	return keeneticLinuxInterfacesFromText(text)
}

type keeneticRouteInterface struct {
	ID          string
	Description string
	Type        string
	Address     string
	Linux       string
}

func parseKeeneticRouteInterfaces(text string) []keeneticRouteInterface {
	var out []keeneticRouteInterface
	var cur *keeneticRouteInterface
	s := bufio.NewScanner(strings.NewReader(text))
	for s.Scan() {
		raw := s.Text()
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "Interface, name =") {
			if cur != nil && cur.ID != "" {
				out = append(out, *cur)
			}
			cur = &keeneticRouteInterface{}
			continue
		}
		if cur == nil {
			continue
		}
		if cur.ID == "" && strings.HasPrefix(line, "id:") {
			cur.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
			continue
		}
		if cur.Type == "" && strings.HasPrefix(line, "type:") {
			cur.Type = strings.TrimSpace(strings.TrimPrefix(line, "type:"))
			continue
		}
		if cur.Description == "" && strings.HasPrefix(line, "description:") {
			cur.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			continue
		}
		if cur.Address == "" && strings.HasPrefix(line, "address:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "address:"))
			if net.ParseIP(v) != nil {
				cur.Address = v
			}
		}
	}
	if cur != nil && cur.ID != "" {
		out = append(out, *cur)
	}
	return out
}

func routeInterfaceIndexFromText(text string) map[string]keeneticRouteInterface {
	items := parseKeeneticRouteInterfaces(text)
	byIP := localInterfaceByIP()
	localNames := make(map[string]bool)
	ifs, _ := net.Interfaces()
	for _, i := range ifs {
		localNames[strings.ToLower(i.Name)] = true
	}
	out := make(map[string]keeneticRouteInterface)
	for _, item := range items {
		if item.Address != "" {
			item.Linux = byIP[item.Address]
		}
		if item.Linux == "" {
			guess := strings.ToLower(item.ID)
			if localNames[guess] {
				item.Linux = guess
			}
		}
		if item.Linux != "" {
			out[item.Linux] = item
		}
	}
	return out
}

func routeInterfaceIndex() (map[string]keeneticRouteInterface, error) {
	text, err := ndmcOutput("show interface", 8*time.Second)
	if err != nil {
		return nil, err
	}
	return routeInterfaceIndexFromText(text), nil
}

func defaultRoutePaths(table int, ifaceIndex map[string]keeneticRouteInterface) []PolicyPathView {
	args := []string{"-4", "route", "show"}
	if table > 0 {
		args = append(args, "table", strconv.Itoa(table))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ip", args...).Output()
	if err != nil {
		return nil
	}
	var paths []PolicyPathView
	seen := make(map[string]bool)
	inDefault := false
	s := bufio.NewScanner(strings.NewReader(string(out)))
	for s.Scan() {
		raw := s.Text()
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "default") {
			inDefault = true
		} else if len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t' {
			inDefault = false
		}
		if !inDefault {
			continue
		}
		fields := strings.Fields(line)
		for i := 0; i < len(fields); i++ {
			if fields[i] != "dev" || i+1 >= len(fields) {
				continue
			}
			dev := fields[i+1]
			weight := 0
			for j := i + 2; j+1 < len(fields); j++ {
				if fields[j] == "weight" {
					weight, _ = strconv.Atoi(fields[j+1])
					break
				}
			}
			key := dev + "/" + strconv.Itoa(weight)
			if seen[key] {
				continue
			}
			seen[key] = true
			p := PolicyPathView{LinuxInterface: dev, Weight: weight}
			if meta, ok := ifaceIndex[dev]; ok {
				p.KeeneticInterface = meta.ID
				p.Description = meta.Description
				p.Type = meta.Type
			}
			paths = append(paths, p)
		}
	}
	return paths
}

func buildPolicyRouteViews(policies map[string]policyRoute, ifaceIndex map[string]keeneticRouteInterface) map[string]PolicyRouteView {
	out := make(map[string]PolicyRouteView)
	out["System"] = PolicyRouteView{Name: "System", Mode: "default", Paths: defaultRoutePaths(0, ifaceIndex)}
	for name, p := range policies {
		out[name] = PolicyRouteView{Name: name, Mode: "policy-mark", Mark: p.Mark, Table: p.Table, Paths: defaultRoutePaths(p.Table, ifaceIndex)}
	}
	return out
}

func discoverPolicyRouteViews() map[string]PolicyRouteView {
	policies := discoverPolicyRoutes()
	ifaceIndex, _ := routeInterfaceIndex()
	return buildPolicyRouteViews(policies, ifaceIndex)
}
