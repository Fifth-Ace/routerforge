package main

// NF3C1: an opt-in, read-only source of community hypotheses. No community
// response is a success signal: the ordinary direct NFQUEUE selector must test
// and independently confirm every selected candidate on this router.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	v2CommunityMaxCandidates = 8
	v2CommunityMaxBody       = 256 * 1024
	v2CommunityMaxRows       = 256
	v2CommunityFetchTimeout  = 12 * time.Second
	v2CommunityFreshDays     = 60
	v2CommunityRaw           = "https://raw.githubusercontent.com/zemidala/nfqws2-strategies/main/data/"
	v2CommunityRipe          = "https://stat.ripe.net/data/"
)

var (
	v2CommunityIDPattern = regexp.MustCompile(`^[a-f0-9]{12,16}$`)
	// Only --lua-desync steps, no file references, shell syntax, or paths.
	v2CommunityFuncPattern  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,40}$`)
	v2CommunityParamPattern = regexp.MustCompile(`^[A-Za-z0-9_.,=+%\-]{1,200}$`)
	v2CommunityHTTP         = &http.Client{
		Timeout:       v2CommunityFetchTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	}
	v2CommunityCache = struct {
		sync.Mutex
		asn        uint32
		asnUntil   time.Time
		itemsASN   uint32
		itemsUntil time.Time
		body       []byte
		warning    string
	}{}
)

type v2CommunityFile struct {
	ASN   uint32 `json:"asn"`
	Items []struct {
		ID      string   `json:"id"`
		ASN     uint32   `json:"asn"`
		Proto   string   `json:"proto"`
		Steps   []string `json:"steps"`
		Reports int      `json:"reports"`
		Fails   int      `json:"fails"`
		Last    string   `json:"last"`
		Targets []struct {
			Host string `json:"host"`
		} `json:"targets"`
	} `json:"items"`
}

type v2CommunityRanked struct {
	Strategy   v2StoredStrategy
	SiteMatch  bool
	Fresh      bool
	NetReports int
	Last       string
}

func v2CommunitySafeStep(step string) bool {
	if !strings.HasPrefix(step, "--lua-desync=") || len(step) > 512 {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(step, "--lua-desync="), ":")
	if len(parts) < 1 || len(parts) > 41 || !v2CommunityFuncPattern.MatchString(parts[0]) {
		return false
	}
	for _, p := range parts[1:] {
		if !v2CommunityParamPattern.MatchString(p) {
			return false
		}
	}
	return true
}

// v2CommunityDecode rejects entire malformed documents and drops malformed
// entries; it never trusts external evidence enough to bypass live testing.
func v2CommunityDecode(body []byte, asn uint32, host string, now time.Time) ([]v2StoredStrategy, error) {
	if asn == 0 || len(body) == 0 || len(body) > v2CommunityMaxBody {
		return nil, errors.New("community response size or ASN is invalid")
	}
	var doc v2CommunityFile
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, errors.New("community response is invalid JSON")
	}
	if doc.ASN != asn || len(doc.Items) > v2CommunityMaxRows {
		return nil, errors.New("community response ASN or entry count mismatch")
	}
	var ranked []v2CommunityRanked
	seen := make(map[string]bool)
	for _, item := range doc.Items {
		if item.ASN != asn || item.Proto != "tls" || !v2CommunityIDPattern.MatchString(item.ID) ||
			len(item.Steps) < 1 || len(item.Steps) > 8 || item.Reports < 1 || item.Fails < 0 ||
			(item.Fails >= 2 && item.Fails > item.Reports) {
			continue
		}
		valid := true
		for _, step := range item.Steps {
			if !v2CommunitySafeStep(step) {
				valid = false
				break
			}
		}
		key := strings.Join(item.Steps, "\x00")
		if !valid || seen[key] {
			continue
		}
		seen[key] = true
		site := false
		for _, target := range item.Targets {
			if strings.EqualFold(strings.TrimSuffix(target.Host, "."), host) {
				site = true
				break
			}
		}
		last, err := time.Parse("2006-01-02", item.Last)
		fresh := err == nil && !last.After(now.Add(24*time.Hour)) && now.Sub(last) <= v2CommunityFreshDays*24*time.Hour
		args := []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello"}
		args = append(args, item.Steps...)
		ranked = append(ranked, v2CommunityRanked{
			Strategy: v2StoredStrategy{
				ID:     "community-" + item.ID,
				Name:   fmt.Sprintf("Community AS%d / %s", asn, item.ID),
				Source: "community", Args: args,
			},
			SiteMatch: site, Fresh: fresh,
			NetReports: item.Reports - item.Fails, Last: item.Last,
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		a, b := ranked[i], ranked[j]
		if a.Fresh != b.Fresh {
			return a.Fresh
		}
		if a.SiteMatch != b.SiteMatch {
			return a.SiteMatch
		}
		if a.NetReports != b.NetReports {
			return a.NetReports > b.NetReports
		}
		return a.Last > b.Last
	})
	if len(ranked) > v2CommunityMaxCandidates {
		ranked = ranked[:v2CommunityMaxCandidates]
	}
	out := make([]v2StoredStrategy, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, item.Strategy)
	}
	return out, nil
}

func v2CommunityGet(ctx context.Context, url string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", "RouterForge-NF3C1")
	resp, err := v2CommunityHTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, v2CommunityMaxBody+1))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if len(data) > v2CommunityMaxBody {
		return resp.StatusCode, nil, errors.New("community response exceeds 256 KiB")
	}
	return resp.StatusCode, data, nil
}

func v2CommunityDetectASN(ctx context.Context) (uint32, error) {
	now := time.Now()
	v2CommunityCache.Lock()
	if v2CommunityCache.asn != 0 && now.Before(v2CommunityCache.asnUntil) {
		a := v2CommunityCache.asn
		v2CommunityCache.Unlock()
		return a, nil
	}
	v2CommunityCache.Unlock()
	code, data, err := v2CommunityGet(ctx, v2CommunityRipe+"whats-my-ip/data.json")
	if err != nil || code != http.StatusOK {
		return 0, errors.New("provider ASN detection failed at RIPEstat")
	}
	var ipReply struct {
		Data struct {
			IP string `json:"ip"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &ipReply) != nil {
		return 0, errors.New("invalid RIPEstat IP response")
	}
	ip := net.ParseIP(ipReply.Data.IP)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsUnspecified() || ip.IsPrivate() {
		return 0, errors.New("RIPEstat returned no public IPv4")
	}
	code, data, err = v2CommunityGet(ctx, v2CommunityRipe+"network-info/data.json?resource="+ip.String())
	if err != nil || code != http.StatusOK {
		return 0, errors.New("provider ASN lookup failed at RIPEstat")
	}
	var netReply struct {
		Data struct {
			ASNs []json.RawMessage `json:"asns"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &netReply) != nil || len(netReply.Data.ASNs) == 0 {
		return 0, errors.New("RIPEstat returned no provider ASN")
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToUpper(strings.Trim(string(netReply.Data.ASNs[0]), `"`)), "AS"), 10, 32)
	if err != nil || n == 0 {
		return 0, errors.New("RIPEstat returned invalid provider ASN")
	}
	asn := uint32(n)
	v2CommunityCache.Lock()
	v2CommunityCache.asn, v2CommunityCache.asnUntil = asn, now.Add(24*time.Hour)
	v2CommunityCache.Unlock()
	return asn, nil
}

// A failed download has a short negative cache. 404 is ordinary: the provider
// simply has no published strategies. At most one provider file is cached.
func v2CommunityCandidates(ctx context.Context, suppliedASN uint32, target string) ([]v2StoredStrategy, string) {
	ctx, cancel := context.WithTimeout(ctx, v2CommunityFetchTimeout)
	defer cancel()
	asn := suppliedASN
	if asn == 0 {
		var err error
		asn, err = v2CommunityDetectASN(ctx)
		if err != nil {
			return nil, "NF3C_COMMUNITY: ASN unavailable; local candidates remain available"
		}
	}
	now := time.Now()
	v2CommunityCache.Lock()
	if v2CommunityCache.itemsASN == asn && now.Before(v2CommunityCache.itemsUntil) {
		data := append([]byte(nil), v2CommunityCache.body...)
		warning := v2CommunityCache.warning
		v2CommunityCache.Unlock()
		if len(data) == 0 {
			return nil, warning
		}
		items, err := v2CommunityDecode(data, asn, strings.ToLower(strings.TrimSuffix(target, ".")), now)
		if err == nil {
			return items, fmt.Sprintf("NF3C_COMMUNITY: AS%d / %d candidates; live NFQUEUE test required", asn, len(items))
		}
		return nil, "NF3C_COMMUNITY: invalid cached source; local candidates remain available"
	}
	v2CommunityCache.Unlock()
	code, data, err := v2CommunityGet(ctx, v2CommunityRaw+"AS"+strconv.FormatUint(uint64(asn), 10)+".json")
	warning := ""
	expiry := 24 * time.Hour
	if err == nil && code == http.StatusNotFound {
		data = nil
		warning = fmt.Sprintf("NF3C_COMMUNITY: AS%d has no shared strategies", asn)
	} else if err == nil && code == http.StatusOK {
		_, err = v2CommunityDecode(data, asn, strings.ToLower(strings.TrimSuffix(target, ".")), now)
		if err == nil {
			warning = fmt.Sprintf("NF3C_COMMUNITY: AS%d / validated hypotheses; live NFQUEUE test required", asn)
		}
	} else if err == nil {
		err = fmt.Errorf("community server returned HTTP %d", code)
	}
	if err != nil {
		data = nil
		warning = "NF3C_COMMUNITY: source unavailable; local candidates remain available"
		expiry = 10 * time.Minute
	}
	v2CommunityCache.Lock()
	v2CommunityCache.itemsASN, v2CommunityCache.itemsUntil = asn, now.Add(expiry)
	v2CommunityCache.body = append([]byte(nil), data...)
	v2CommunityCache.warning = warning
	v2CommunityCache.Unlock()
	if len(data) == 0 {
		return nil, warning
	}
	items, err := v2CommunityDecode(data, asn, strings.ToLower(strings.TrimSuffix(target, ".")), now)
	if err != nil {
		return nil, "NF3C_COMMUNITY: source validation failed; local candidates remain available"
	}
	return items, fmt.Sprintf("NF3C_COMMUNITY: AS%d / %d candidates; live NFQUEUE test required", asn, len(items))
}
