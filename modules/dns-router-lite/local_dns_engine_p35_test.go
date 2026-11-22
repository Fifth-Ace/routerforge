package lite

import (
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
	"testing"
)

func TestP35ExactRecordsAndStockFallback(t *testing.T) {
	cfg := &dr.LocalDNSConfig{Enabled: true, Records: []dr.LocalDNSRecord{
		{Name: "nas.rf.home.arpa", Type: "A", Value: "192.168.10.8", TTL: 300},
		{Name: "alias.rf.home.arpa", Type: "CNAME", Value: "nas.rf.home.arpa", TTL: 300},
		{Name: "8.10.168.192.in-addr.arpa", Type: "PTR", Value: "nas.rf.home.arpa", TTL: 300},
	}}
	records := compileLocalRecordsP35(cfg)
	for _, tc := range []struct {
		name    string
		query   uint16
		handled bool
		rcode   int
		answers int
	}{
		{"nas.rf.home.arpa.", dns.TypeA, true, dns.RcodeSuccess, 1},
		{"alias.rf.home.arpa.", dns.TypeA, true, dns.RcodeSuccess, 2},
		{"nas.rf.home.arpa.", dns.TypeAAAA, true, dns.RcodeSuccess, 0},
		{"missing.rf.home.arpa.", dns.TypeA, true, dns.RcodeNameError, 0},
		{"8.10.168.192.in-addr.arpa.", dns.TypePTR, true, dns.RcodeSuccess, 1},
		{"9.10.168.192.in-addr.arpa.", dns.TypePTR, false, 0, 0},
		{"router.lan.", dns.TypeA, false, 0, 0},
		{"my.keenetic.net.", dns.TypeA, false, 0, 0},
	} {
		q := new(dns.Msg)
		q.SetQuestion(tc.name, tc.query)
		res, ok := records.answer(q)
		if ok != tc.handled {
			t.Errorf("%s handled=%v", tc.name, ok)
			continue
		}
		if ok && (res.Rcode != tc.rcode || len(res.Answer) != tc.answers) {
			t.Errorf("%s rcode=%d answers=%d", tc.name, res.Rcode, len(res.Answer))
		}
	}
	disabled := compileLocalRecordsP35(nil)
	q := new(dns.Msg)
	q.SetQuestion("nas.rf.home.arpa.", dns.TypeA)
	if _, ok := disabled.answer(q); ok {
		t.Fatal("disabled engine served local DNS")
	}
}
