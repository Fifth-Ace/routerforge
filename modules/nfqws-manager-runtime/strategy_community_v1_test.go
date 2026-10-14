package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func nf3cFile(asn uint32, items ...map[string]any) []byte {
	data, _ := json.Marshal(map[string]any{"asn": asn, "items": items})
	return data
}
func nf3cRow(id, step, host string, reports, fails int, last string) map[string]any {
	return map[string]any{
		"id": id, "asn": 12389, "proto": "tls", "steps": []string{step},
		"reports": reports, "fails": fails, "last": last,
		"targets": []map[string]any{{"host": host}},
	}
}
func TestNF3CCommunityRealSchemaAndSameSiteRank(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	older := nf3cRow("111111111111", "--lua-desync=fake:blob=tls_clienthello:badsum", "other.test", 3, 0, "2026-10-11")
	matching := nf3cRow("222222222222", "--lua-desync=hostfakesplit:host=ya.ru:tcp_md5", "target.test", 1, 0, "2026-10-12")
	items, err := v2CommunityDecode(nf3cFile(12389, older, matching), 12389, "target.test", now)
	if err != nil || len(items) != 2 || items[0].ID != "community-222222222222" {
		t.Fatalf("ranking: %v %+v", err, items)
	}
	if items[0].Source != "community" || len(items[0].Args) != 4 || items[0].Args[0] != "--filter-tcp=443" {
		t.Fatalf("candidate not portable: %+v", items[0])
	}
}
func TestNF3CRejectsUnsafeSteps(t *testing.T) {
	bad := []string{
		"--lua-desync=fake:blob=@/etc/passwd", "--lua-desync=fake;reboot",
		"--lua-desync=fake:blob=$(id)", "--new", "--qnum=3",
		"--lua-desync=fake:blob=foo/bar", "--lua-desync=fake:blob=x\n--daemon",
	}
	for n, s := range bad {
		row := nf3cRow(fmt.Sprintf("%012x", n+1), s, "x.test", 1, 0, "2026-10-13")
		got, err := v2CommunityDecode(nf3cFile(12389, row), 12389, "x.test", time.Now())
		if err != nil || len(got) != 0 {
			t.Fatalf("unsafe %q accepted: %v %+v", s, err, got)
		}
	}
}
func TestNF3CRejectsWrongASNAndHTTP(t *testing.T) {
	row := nf3cRow("222222222222", "--lua-desync=fake:blob=tls_clienthello", "x.test", 1, 0, "2026-10-12")
	if _, err := v2CommunityDecode(nf3cFile(12389, row), 25513, "x.test", time.Now()); err == nil {
		t.Fatal("file ASN mismatch accepted")
	}
	row["asn"] = 25513
	x, err := v2CommunityDecode(nf3cFile(12389, row), 12389, "x.test", time.Now())
	if err != nil || len(x) > 0 {
		t.Fatal("row ASN mismatch accepted")
	}
	row["asn"] = uint32(12389)
	row["proto"] = "quic"
	x, err = v2CommunityDecode(nf3cFile(12389, row), 12389, "x.test", time.Now())
	if err != nil || len(x) > 0 {
		t.Fatal("non TLS accepted")
	}
}
func TestNF3CBlocksUnsupportedOrFailedMajority(t *testing.T) {
	row := nf3cRow("333333333333", "--lua-desync=fake:blob=tls_clienthello", "x.test", 1, 2, "2026-10-12")
	got, err := v2CommunityDecode(nf3cFile(12389, row), 12389, "x.test", time.Now())
	if err != nil || len(got) != 0 {
		t.Fatal("failed-majority accepted")
	}
	row["fails"] = 0
	row["steps"] = []string{"--lua-desync=fake:blob=tls_clienthello", "--lua-desync=fake:blob=@disk"}
	got, err = v2CommunityDecode(nf3cFile(12389, row), 12389, "x.test", time.Now())
	if err != nil || len(got) != 0 {
		t.Fatal("partially invalid steps accepted")
	}
}
func TestNF3CCapsAndDeduplicates(t *testing.T) {
	var rows []map[string]any
	for i := 1; i <= 20; i++ {
		rows = append(rows, nf3cRow(fmt.Sprintf("%012x", i), fmt.Sprintf("--lua-desync=fake:repeats=%d", i), "site.test", 1, 0, "2026-10-12"))
	}
	got, err := v2CommunityDecode(nf3cFile(12389, rows...), 12389, "site.test", time.Now())
	if err != nil || len(got) != 8 {
		t.Fatalf("uncapped: %d %v", len(got), err)
	}
	got, err = v2CommunityDecode(nf3cFile(12389, rows[0], rows[0]), 12389, "site.test", time.Now())
	if err != nil || len(got) != 1 {
		t.Fatalf("duplicate: %d %v", len(got), err)
	}
}
func TestNF3CStrictBodyAndRowLimits(t *testing.T) {
	if _, err := v2CommunityDecode([]byte("{"+strings.Repeat(" ", v2CommunityMaxBody)+"}"), 12389, "x.test", time.Now()); err == nil {
		t.Fatal("body unbounded")
	}
	rows := make([]map[string]any, v2CommunityMaxRows+1)
	for i := range rows {
		rows[i] = nf3cRow(fmt.Sprintf("%012x", i+1), "--lua-desync=fake:blob=tls_clienthello", "x.test", 1, 0, "2026-10-12")
	}
	if _, err := v2CommunityDecode(nf3cFile(12389, rows...), 12389, "x.test", time.Now()); err == nil {
		t.Fatal("rows unbounded")
	}
}
func TestNF3CFreshnessRanksAboveOld(t *testing.T) {
	now := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	old := nf3cRow("aaaaaaaaaaaa", "--lua-desync=fake:repeats=1", "site.test", 10, 0, "2026-01-01")
	recent := nf3cRow("bbbbbbbbbbbb", "--lua-desync=fake:repeats=2", "other.test", 1, 0, "2026-10-13")
	got, err := v2CommunityDecode(nf3cFile(12389, old, recent), 12389, "site.test", now)
	if err != nil || len(got) != 2 || got[0].ID != "community-bbbbbbbbbbbb" {
		t.Fatalf("stale priority: %+v %v", got, err)
	}
}

func TestNF3CActualUpstreamStepGrammar(t *testing.T) {
	examples := []string{
		"--lua-desync=fake:blob=stun_fake:tcp_ts=-1500:badsum",
		"--lua-desync=fake:blob=tls_clienthello_sochi_park:tls_mod=rnd,dupsid:tcp_ts=-1500:badsum",
		"--lua-desync=multisplit:pos=1,sniext-2:seqovl=1:seqovl_pattern=tls_clienthello_sochi_park",
		"--lua-desync=hostfakesplit:midhost=host-2:seqovl=sniext+3:seqovl_pattern=tls_clienthello:badsum:tcp_md5",
	}
	for _, step := range examples {
		if !v2CommunitySafeStep(step) {
			t.Fatalf("upstream step rejected: %s", step)
		}
	}
}

func TestNF3CSharedProviderCacheRanksEachTargetIndependently(t *testing.T) {
	a := nf3cRow("aaaaaaaaaaaa", "--lua-desync=fake:blob=tls_clienthello:repeats=1", "alpha.test", 1, 0, "2026-10-13")
	b := nf3cRow("bbbbbbbbbbbb", "--lua-desync=fake:blob=tls_clienthello:repeats=2", "beta.test", 1, 0, "2026-10-13")
	v2CommunityCache.Lock()
	v2CommunityCache.itemsASN = 12389
	v2CommunityCache.itemsUntil = time.Now().Add(time.Hour)
	v2CommunityCache.body = nf3cFile(12389, a, b)
	v2CommunityCache.warning = "cached"
	v2CommunityCache.Unlock()
	defer func() {
		v2CommunityCache.Lock()
		v2CommunityCache.itemsUntil = time.Time{}
		v2CommunityCache.body = nil
		v2CommunityCache.Unlock()
	}()
	x, _ := v2CommunityCandidates(context.Background(), 12389, "alpha.test")
	y, _ := v2CommunityCandidates(context.Background(), 12389, "beta.test")
	if len(x) != 2 || len(y) != 2 || x[0].ID == y[0].ID {
		t.Fatalf("site-specific rank polluted by cache: %+v %+v", x, y)
	}
}

func TestNF3CTransportRejectsRedirectAndLargePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://example.org/", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("a", v2CommunityMaxBody+9)))
	}))
	defer server.Close()
	status, _, err := v2CommunityGet(context.Background(), server.URL+"/redirect")
	if err != nil || status != http.StatusFound {
		t.Fatalf("redirect was followed: %d %v", status, err)
	}
	_, _, err = v2CommunityGet(context.Background(), server.URL+"/big")
	if err == nil {
		t.Fatal("oversized feed accepted")
	}
}
