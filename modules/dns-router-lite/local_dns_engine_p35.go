package lite

import (
	"net"
	"strings"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

// localRecordsP35 is immutable after engine initialization.
type localRecordsP35 struct {
	enabled bool
	values  map[string]map[string]dr.LocalDNSRecord
}

func compileLocalRecordsP35(cfg *dr.LocalDNSConfig) *localRecordsP35 {
	store := &localRecordsP35{values: make(map[string]map[string]dr.LocalDNSRecord)}
	if cfg == nil || !cfg.Enabled {
		return store
	}
	store.enabled = true
	for _, r := range cfg.Records {
		name := strings.ToLower(r.Name)
		if store.values[name] == nil {
			store.values[name] = make(map[string]dr.LocalDNSRecord)
		}
		store.values[name][r.Type] = r
	}
	return store
}

func p35RR(name string, r dr.LocalDNSRecord) dns.RR {
	hdr := dns.RR_Header{Name: dns.Fqdn(name), Rrtype: dns.StringToType[r.Type], Class: dns.ClassINET, Ttl: r.TTL}
	switch r.Type {
	case "A":
		return &dns.A{Hdr: hdr, A: net.ParseIP(r.Value).To4()}
	case "AAAA":
		return &dns.AAAA{Hdr: hdr, AAAA: net.ParseIP(r.Value).To16()}
	case "CNAME":
		return &dns.CNAME{Hdr: hdr, Target: dns.Fqdn(r.Value)}
	case "PTR":
		return &dns.PTR{Hdr: hdr, Ptr: dns.Fqdn(r.Value)}
	}
	return nil
}

func (l *localRecordsP35) answer(req *dns.Msg) (*dns.Msg, bool) {
	if l == nil || !l.enabled || req == nil || len(req.Question) != 1 {
		return nil, false
	}
	q := req.Question[0]
	if q.Qclass != dns.ClassINET {
		return nil, false
	}
	name := strings.TrimSuffix(strings.ToLower(q.Name), ".")
	kind := ""
	if name != "rf.home.arpa" && strings.HasSuffix(name, ".rf.home.arpa") {
		kind = "zone"
	}
	if q.Qtype == dns.TypePTR && l.values[name]["PTR"].Type == "PTR" {
		kind = "ptr"
	}
	if kind == "" {
		return nil, false
	}
	answer := new(dns.Msg)
	answer.SetReply(req)
	answer.Authoritative = true
	if kind == "ptr" {
		answer.Answer = []dns.RR{p35RR(name, l.values[name]["PTR"])}
		return answer, true
	}
	types := l.values[name]
	if len(types) == 0 {
		answer.Rcode = dns.RcodeNameError
		return answer, true
	}
	wanted := dns.TypeToString[q.Qtype]
	if r, ok := types[wanted]; ok {
		answer.Answer = append(answer.Answer, p35RR(name, r))
		return answer, true
	}
	// A/AAAA queries follow only explicitly configured local CNAME records.
	if q.Qtype == dns.TypeA || q.Qtype == dns.TypeAAAA {
		current := name
		for i := 0; i <= dr.P35MaxRecords; i++ {
			records := l.values[current]
			alias, ok := records["CNAME"]
			if !ok {
				break
			}
			answer.Answer = append(answer.Answer, p35RR(current, alias))
			current = alias.Value
			if addr, ok := l.values[current][wanted]; ok {
				answer.Answer = append(answer.Answer, p35RR(current, addr))
				break
			}
		}
	}
	return answer, true
}
