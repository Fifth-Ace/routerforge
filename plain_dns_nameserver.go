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

	flush := func() {
		rawAddress := strings.TrimSpace(address)
		rawDomain := strings.TrimSpace(domain)
		address, domain = "", ""

		ip, port, ok := parsePlainDNSEndpoint(rawAddress)
		if !ok {
			return
		}
		if rawDomain == "." {
			rawDomain = ""
		}

		key := plainDNSEndpoint(ip, port)
		item := byEndpoint[key]
		if item == nil {
			item = &PlainDNSMeta{
				Address: ip,
				Port:    port,
				Name:    friendlyPlainDNSName(ip),
			}
			byEndpoint[key] = item
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
		}
	}
	flush()

	out := make([]PlainDNSMeta, 0, len(byEndpoint))
	for _, item := range byEndpoint {
		item.Domains = uniqueSorted(item.Domains)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Address != out[j].Address {
			return out[i].Address < out[j].Address
		}
		return out[i].Port < out[j].Port
	})
	return out
}