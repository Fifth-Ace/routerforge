package lite

// Original preserves the DNS response, but an explicitly enabled IP health
// monitor must still receive observations. Measurements never block a query.

import (
	"net/netip"

	"github.com/Fifth-Ace/routerforge/modules/dns-router-lite/internal/ipintel"
	"github.com/miekg/dns"
)

func applyIPIntelligenceP26(planner *ipintel.Planner, monitor *IPHealthMonitor, request, answer *dns.Msg, mode ipintel.Mode, key, verifiedMAC string) *dns.Msg {
	var metrics map[netip.Addr]ipintel.Health
	if monitor != nil {
		metrics = monitor.Observe(request, answer)
	}
	if mode == "" || mode == ipintel.Original {
		return answer
	}
	return reorderDNSAnswer(planner, request, answer, mode, key, verifiedMAC, metrics)
}
