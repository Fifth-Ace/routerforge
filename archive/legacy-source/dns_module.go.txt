package main

import "os"

const (
	routerForgeDNSMarker = "/opt/etc/routerforge/dns.enabled"
	legacyDNSMarker      = "/opt/etc/dns-monitor/dns.enabled"
)

func dnsModuleEnabled() bool {
	for _, marker := range []string{routerForgeDNSMarker, legacyDNSMarker} {
		if _, err := os.Stat(marker); err == nil {
			return true
		}
	}
	return false
}
