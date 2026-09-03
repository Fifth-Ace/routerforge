package main

import (
	"sort"
	"strings"
)

func parseIPNameServers(text string) []PlainDNSMeta {
	lines := strings.Split(text, "\n")
	byEndpoint := map[string]*PlainDNSMeta{}
	address := ""
	domain := ""
	service := ""
	iface := ""

	flush := func() {
		rawAddress := strings.TrimSpace(address)
		rawDomain := strings.TrimSpace(domain)
		rawService := strings.TrimSpace(service)
		rawInterface := strings.TrimSpace(iface)
		address, domain, service, iface = "", "", "", ""

		ip, port, ok := parsePlainDNSEndpoint(rawAddress)
		if !ok {
			return
		}
		if rawDomain == "." {
			rawDomain = ""
		}

		source := rawService
		if strings.HasPrefix(strings.ToLower(rawService), "dhcp::") {
			source = "DHCP"
		}

		key := plainDNSEndpoint(ip, port)
		item := byEndpoint[key]
		if item == nil {
			item = &PlainDNSMeta{
				Address:   ip,
				Port:      port,
				Name:      friendlyPlainDNSName(ip),
				Source:    source,
				Interface: rawInterface,
			}
			byEndpoint[key] = item
		} else {
			if item.Source == "" {
				item.Source = source
			}
			if item.Interface == "" {
				item.Interface = rawInterface
			}
		}
		if rawDomain != "" {
			item.Domains = append(item.Domains, rawDomain)
		}
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "server:" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "address:"):
			address = strings.TrimSpace(strings.TrimPrefix(line, "address:"))
		case strings.HasPrefix(line, "domain:"):
			domain = strings.TrimSpace(strings.TrimPrefix(line, "domain:"))
		case strings.HasPrefix(line, "service:"):
			service = strings.TrimSpace(strings.TrimPrefix(line, "service:"))
		case strings.HasPrefix(line, "interface:"):
			iface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
	}
	flush()

	out := make([]PlainDNSMeta, 0, len(byEndpoint))
	for _, item := range byEndpoint {
		item.Domains = uniqueSorted(item.Domains)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			if out[i].Source == "DHCP" {
				return true
			}
			if out[j].Source == "DHCP" {
				return false
			}
		}
		if out[i].Address != out[j].Address {
			return out[i].Address < out[j].Address
		}
		return out[i].Port < out[j].Port
	})
	return out
}
