package main

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RouterDNSInfoSnapshot struct {
	GeneratedAt   time.Time                   `json:"generated_at"`
	Proxies       []RouterDNSProxyInfo        `json:"proxies"`
	StaticRecords []RouterDNSStaticRecordInfo `json:"static_records"`
	Rebind        RouterDNSRebindInfo         `json:"rebind"`
}

type RouterDNSProxyInfo struct {
	Name      string                  `json:"name"`
	TCPPort   int                     `json:"tcp_port"`
	UDPPort   int                     `json:"udp_port"`
	Stat      RouterDNSProxyStatInfo  `json:"stat"`
	Upstreams []RouterDNSUpstreamInfo `json:"upstreams"`
}

type RouterDNSProxyStatInfo struct {
	TotalRequests     int     `json:"total_requests"`
	ProxyRequestsSent int     `json:"proxy_requests_sent"`
	CacheHitRatio     float64 `json:"cache_hit_ratio"`
	CacheHits         int     `json:"cache_hits"`
	Memory            string  `json:"memory"`
}

type RouterDNSUpstreamInfo struct {
	Name      string `json:"name"`
	Protocol  string `json:"protocol"`
	Address   string `json:"address"`
	Port      int    `json:"port"`
	Target    string `json:"target"`
	SNI       string `json:"sni"`
	Interface string `json:"interface"`
	Domain    string `json:"domain"`
	LocalPort int    `json:"local_port"`
	RSent     int    `json:"r_sent"`
	ARcvd     int    `json:"a_rcvd"`
	NXRcvd    int    `json:"nx_rcvd"`
	MedResp   string `json:"med_resp"`
	AvgResp   string `json:"avg_resp"`
	Rank      int    `json:"rank"`
}

type RouterDNSStaticRecordInfo struct {
	Host  string `json:"host"`
	Type  string `json:"type"`
	Value string `json:"value"`
	Flag  int    `json:"flag"`
}

type RouterDNSRebindInfo struct {
	Enabled  bool     `json:"enabled"`
	Nets     []string `json:"nets"`
	Excludes []string `json:"excludes"`
}

type dnsInfoServerStat struct {
	RSent   int
	ARcvd   int
	NXRcvd  int
	MedResp string
	AvgResp string
	Rank    int
}

type dnsInfoTLSMeta struct {
	Address   string
	Port      int
	SNI       string
	Interface string
	Domain    string
}

type dnsInfoHTTPSMeta struct {
	URI       string
	Interface string
	Domain    string
}

func readDNSInfo() (RouterDNSInfoSnapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ndmc", "-c", "show dns-proxy").Output()
	if err != nil {
		return RouterDNSInfoSnapshot{}, fmt.Errorf("ndmc show dns-proxy: %w", err)
	}
	info := parseDNSInfo(string(out))
	info.GeneratedAt = time.Now()
	return info, nil
}

func parseDNSInfo(text string) RouterDNSInfoSnapshot {
	var result RouterDNSInfoSnapshot
	var current *RouterDNSProxyInfo
	serverStats := map[string]map[int]dnsInfoServerStat{}
	tlsMeta := map[string][]dnsInfoTLSMeta{}
	httpsMeta := map[string][]dnsInfoHTTPSMeta{}
	inServerTable := false
	tlsIndex := -1
	httpsIndex := -1

	flushProxy := func() {
		if current == nil {
			return
		}
		if stats := serverStats[current.Name]; stats != nil {
			for i := range current.Upstreams {
				if row, ok := stats[current.Upstreams[i].LocalPort]; ok {
					current.Upstreams[i].RSent = row.RSent
					current.Upstreams[i].ARcvd = row.ARcvd
					current.Upstreams[i].NXRcvd = row.NXRcvd
					current.Upstreams[i].MedResp = row.MedResp
					current.Upstreams[i].AvgResp = row.AvgResp
					current.Upstreams[i].Rank = row.Rank
				}
			}
		}
		result.Proxies = append(result.Proxies, *current)
		current = nil
		inServerTable = false
	}

	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "proxy-name:") {
			flushProxy()
			name := strings.TrimSpace(strings.TrimPrefix(line, "proxy-name:"))
			current = &RouterDNSProxyInfo{Name: name}
			serverStats[name] = map[int]dnsInfoServerStat{}
			tlsMeta[name] = nil
			httpsMeta[name] = nil
			tlsIndex = -1
			httpsIndex = -1
			continue
		}
		if current == nil || line == "" {
			continue
		}

		switch line {
		case "proxy-config:", "proxy-stat:", "proxy-tls:", "proxy-https:":
			tlsIndex = -1
			httpsIndex = -1
			inServerTable = false
			continue
		case "server-tls:":
			tlsMeta[current.Name] = append(tlsMeta[current.Name], dnsInfoTLSMeta{})
			tlsIndex = len(tlsMeta[current.Name]) - 1
			httpsIndex = -1
			inServerTable = false
			continue
		case "server-https:":
			httpsMeta[current.Name] = append(httpsMeta[current.Name], dnsInfoHTTPSMeta{})
			httpsIndex = len(httpsMeta[current.Name]) - 1
			tlsIndex = -1
			inServerTable = false
			continue
		}

		if tlsIndex >= 0 {
			meta := &tlsMeta[current.Name][tlsIndex]
			switch {
			case strings.HasPrefix(line, "address:"):
				meta.Address = strings.TrimSpace(strings.TrimPrefix(line, "address:"))
				continue
			case strings.HasPrefix(line, "port:"):
				meta.Port = dnsInfoAtoi(strings.TrimSpace(strings.TrimPrefix(line, "port:")))
				continue
			case strings.HasPrefix(line, "sni:"):
				meta.SNI = strings.TrimSpace(strings.TrimPrefix(line, "sni:"))
				continue
			case strings.HasPrefix(line, "interface:"):
				meta.Interface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
				continue
			case strings.HasPrefix(line, "domain:"):
				meta.Domain = strings.TrimSpace(strings.TrimPrefix(line, "domain:"))
				continue
			}
		}
		if httpsIndex >= 0 {
			meta := &httpsMeta[current.Name][httpsIndex]
			switch {
			case strings.HasPrefix(line, "uri:"):
				meta.URI = strings.TrimSpace(strings.TrimPrefix(line, "uri:"))
				continue
			case strings.HasPrefix(line, "interface:"):
				meta.Interface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
				continue
			case strings.HasPrefix(line, "domain:"):
				meta.Domain = strings.TrimSpace(strings.TrimPrefix(line, "domain:"))
				continue
			case strings.HasPrefix(line, "format:"):
				continue
			}
		}

		switch {
		case strings.HasPrefix(line, "dns_tcp_port"):
			current.TCPPort = dnsInfoAtoi(dnsInfoAfterEquals(line))
			inServerTable = false
			continue
		case strings.HasPrefix(line, "dns_udp_port"):
			current.UDPPort = dnsInfoAtoi(dnsInfoAfterEquals(line))
			inServerTable = false
			continue
		case strings.HasPrefix(line, "dns_server"):
			if upstream, ok := parseDNSInfoServer(line); ok {
				current.Upstreams = append(current.Upstreams, upstream)
			}
			inServerTable = false
			continue
		case strings.HasPrefix(line, "static_aaaa"):
			if record, ok := parseDNSInfoStatic(line, "AAAA"); ok {
				result.StaticRecords = append(result.StaticRecords, record)
			}
			inServerTable = false
			continue
		case strings.HasPrefix(line, "static_a "), strings.HasPrefix(line, "static_a="):
			if record, ok := parseDNSInfoStatic(line, "A"); ok {
				result.StaticRecords = append(result.StaticRecords, record)
			}
			inServerTable = false
			continue
		case strings.HasPrefix(line, "norebind_ctl"):
			result.Rebind.Enabled = strings.EqualFold(dnsInfoAfterEquals(line), "on")
			inServerTable = false
			continue
		case strings.HasPrefix(line, "norebind_ip4net"):
			if value := dnsInfoAfterEquals(line); value != "" {
				result.Rebind.Nets = append(result.Rebind.Nets, value)
			}
			inServerTable = false
			continue
		case strings.HasPrefix(line, "norebind_exclude"):
			value := dnsInfoAfterEquals(line)
			if value == "" {
				parts := strings.Fields(line)
				if len(parts) > 1 {
					value = parts[len(parts)-1]
				}
			}
			if value != "" {
				result.Rebind.Excludes = append(result.Rebind.Excludes, value)
			}
			inServerTable = false
			continue
		case strings.HasPrefix(line, "Total incoming requests:"):
			current.Stat.TotalRequests = dnsInfoAtoi(dnsInfoAfterColon(line))
			continue
		case strings.HasPrefix(line, "Proxy requests sent:"):
			current.Stat.ProxyRequestsSent = dnsInfoAtoi(dnsInfoAfterColon(line))
			continue
		case strings.HasPrefix(line, "Cache hits ratio:"):
			rest := dnsInfoAfterColon(line)
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				current.Stat.CacheHitRatio, _ = strconv.ParseFloat(fields[0], 64)
			}
			if open := strings.Index(rest, "("); open >= 0 {
				inner := strings.Trim(strings.TrimSpace(rest[open+1:]), ")")
				current.Stat.CacheHits = dnsInfoAtoi(inner)
			}
			continue
		case strings.HasPrefix(line, "Memory usage:"):
			current.Stat.Memory = dnsInfoAfterColon(line)
			continue
		case strings.HasPrefix(line, "Ip") && strings.Contains(line, "Port"):
			inServerTable = true
			continue
		}

		if inServerTable {
			fields := strings.Fields(line)
			if len(fields) >= 8 {
				port := dnsInfoAtoi(fields[1])
				if port > 0 {
					serverStats[current.Name][port] = dnsInfoServerStat{
						RSent:   dnsInfoAtoi(fields[2]),
						ARcvd:   dnsInfoAtoi(fields[3]),
						NXRcvd:  dnsInfoAtoi(fields[4]),
						MedResp: fields[5],
						AvgResp: fields[6],
						Rank:    dnsInfoAtoi(fields[7]),
					}
					continue
				}
			}
		}
	}
	flushProxy()

	for pi := range result.Proxies {
		proxy := &result.Proxies[pi]
		for ui := range proxy.Upstreams {
			u := &proxy.Upstreams[ui]
			switch u.Protocol {
			case "DoT":
				for _, meta := range tlsMeta[proxy.Name] {
					port := meta.Port
					if port == 0 {
						port = 853
					}
					if meta.Address != u.Address || port != u.Port {
						continue
					}
					if meta.SNI != "" {
						u.SNI = meta.SNI
					}
					u.Interface = meta.Interface
					if u.Domain == "" {
						u.Domain = meta.Domain
					}
					break
				}
			case "DoH":
				for _, meta := range httpsMeta[proxy.Name] {
					if dnsInfoNormalizeURI(meta.URI) != dnsInfoNormalizeURI(u.Target) {
						continue
					}
					u.Interface = meta.Interface
					if u.Domain == "" {
						u.Domain = meta.Domain
					}
					break
				}
			}
		}
	}

	result.StaticRecords = dnsInfoUniqueStatic(result.StaticRecords)
	result.Rebind.Nets = dnsInfoUniqueStrings(result.Rebind.Nets)
	result.Rebind.Excludes = dnsInfoUniqueStrings(result.Rebind.Excludes)
	return result
}

func parseDNSInfoServer(line string) (RouterDNSUpstreamInfo, bool) {
	value := dnsInfoAfterEquals(line)
	if value == "" {
		return RouterDNSUpstreamInfo{}, false
	}

	comment := ""
	if index := strings.Index(value, "#"); index >= 0 {
		comment = strings.TrimSpace(value[index+1:])
		value = strings.TrimSpace(value[:index])
	}
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return RouterDNSUpstreamInfo{}, false
	}

	domain := ""
	if len(fields) > 1 && fields[1] != "." {
		domain = strings.TrimSpace(fields[1])
	}

	upstream := RouterDNSUpstreamInfo{Domain: domain}
	if comment == "" {
		host, port := dnsInfoSplitEndpoint(fields[0], 53)
		if host == "" {
			return RouterDNSUpstreamInfo{}, false
		}
		upstream.Protocol = "DNS"
		upstream.Address = host
		upstream.Port = port
		upstream.LocalPort = port
		upstream.Target = host
		upstream.Name = dnsInfoFriendlyPlainName(host)
		return upstream, true
	}

	_, localPort := dnsInfoSplitEndpoint(fields[0], 0)
	upstream.LocalPort = localPort

	if strings.HasPrefix(comment, "https://") || strings.HasPrefix(comment, "http://") {
		raw := comment
		if at := strings.LastIndex(raw, "@"); at > 0 {
			suffix := raw[at+1:]
			if suffix != "" && !strings.ContainsAny(suffix, "/:.") {
				raw = raw[:at]
			}
		}
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Hostname() == "" {
			return RouterDNSUpstreamInfo{}, false
		}
		port := 443
		if parsed.Port() != "" {
			port = dnsInfoAtoi(parsed.Port())
		} else if parsed.Scheme == "http" {
			port = 80
		}
		upstream.Protocol = "DoH"
		upstream.Address = parsed.Hostname()
		upstream.Port = port
		upstream.Target = raw
		upstream.SNI = parsed.Hostname()
		upstream.Name = dnsInfoFriendlyResolverName(parsed.Hostname(), "DoH")
		return upstream, true
	}

	addressPart := comment
	if at := strings.Index(comment, "@"); at >= 0 {
		addressPart = strings.TrimSpace(comment[:at])
		upstream.SNI = strings.TrimSpace(comment[at+1:])
	}
	host, port := dnsInfoSplitEndpoint(addressPart, 853)
	if host == "" {
		return RouterDNSUpstreamInfo{}, false
	}
	upstream.Protocol = "DoT"
	upstream.Address = host
	upstream.Port = port
	upstream.Target = addressPart
	if upstream.SNI == "" {
		upstream.SNI = host
	}
	upstream.Name = dnsInfoFriendlyResolverName(upstream.SNI, "DoT")
	return upstream, true
}

func parseDNSInfoStatic(line, recordType string) (RouterDNSStaticRecordInfo, bool) {
	fields := strings.Fields(dnsInfoAfterEquals(line))
	if len(fields) < 2 {
		return RouterDNSStaticRecordInfo{}, false
	}
	record := RouterDNSStaticRecordInfo{Host: fields[0], Type: recordType, Value: fields[1]}
	if len(fields) > 2 {
		record.Flag = dnsInfoAtoi(fields[2])
	}
	return record, true
}

func dnsInfoTargetEndpoint(protocol, target string) (string, int) {
	if protocol == "DoH" {
		parsed, err := url.Parse(target)
		if err != nil || parsed.Hostname() == "" {
			return "", 0
		}
		port := 443
		if parsed.Port() != "" {
			port = dnsInfoAtoi(parsed.Port())
		} else if parsed.Scheme == "http" {
			port = 80
		}
		return parsed.Hostname(), port
	}
	if protocol == "DoT" {
		return dnsInfoSplitEndpoint(target, 853)
	}
	return dnsInfoSplitEndpoint(target, 53)
}

func dnsInfoSplitEndpoint(raw string, defaultPort int) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", defaultPort
	}
	if host, portString, err := net.SplitHostPort(raw); err == nil {
		return strings.Trim(host, "[]"), dnsInfoAtoi(portString)
	}
	if ip := net.ParseIP(strings.Trim(raw, "[]")); ip != nil {
		return ip.String(), defaultPort
	}
	if strings.Count(raw, ":") == 1 {
		host, portString, ok := strings.Cut(raw, ":")
		if ok && dnsInfoAtoi(portString) > 0 {
			return strings.TrimSpace(host), dnsInfoAtoi(portString)
		}
	}
	return strings.Trim(raw, "[]"), defaultPort
}

func dnsInfoAfterEquals(line string) string {
	if index := strings.Index(line, "="); index >= 0 {
		return strings.TrimSpace(line[index+1:])
	}
	return ""
}

func dnsInfoAfterColon(line string) string {
	if index := strings.Index(line, ":"); index >= 0 {
		return strings.TrimSpace(line[index+1:])
	}
	return ""
}

func dnsInfoAtoi(value string) int {
	value = strings.TrimSpace(strings.Trim(value, "()"))
	number, _ := strconv.Atoi(value)
	return number
}

func dnsInfoNormalizeURI(value string) string {
	value = strings.TrimSpace(value)
	if at := strings.LastIndex(value, "@"); at > 0 {
		suffix := value[at+1:]
		if suffix != "" && !strings.ContainsAny(suffix, "/:.") {
			value = value[:at]
		}
	}
	return strings.TrimSuffix(value, "/")
}

func dnsInfoFriendlyResolverName(host, protocol string) string {
	lower := strings.ToLower(host)
	switch {
	case strings.Contains(lower, "quad9"):
		return "Quad9 " + protocol
	case lower == "dns.google" || strings.Contains(lower, "google"):
		return "Google " + protocol
	case strings.Contains(lower, "cloudflare"):
		return "Cloudflare " + protocol
	case strings.Contains(lower, "yandex"):
		return "Yandex " + protocol
	case strings.Contains(lower, "comss"):
		return "Comss " + protocol
	case strings.Contains(lower, "astracat"):
		return "Astracat " + protocol
	}
	if host != "" {
		return host + " " + protocol
	}
	return protocol
}

func dnsInfoFriendlyPlainName(host string) string {
	switch host {
	case "1.1.1.1", "1.0.0.1":
		return "Cloudflare DNS"
	case "8.8.8.8", "8.8.4.4":
		return "Google DNS"
	case "9.9.9.9", "149.112.112.112":
		return "Quad9 DNS"
	case "77.88.8.8", "77.88.8.1":
		return "Yandex DNS"
	}
	return host
}

func dnsInfoUniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func dnsInfoUniqueStatic(records []RouterDNSStaticRecordInfo) []RouterDNSStaticRecordInfo {
	seen := map[string]struct{}{}
	out := make([]RouterDNSStaticRecordInfo, 0, len(records))
	for _, record := range records {
		key := strings.ToLower(fmt.Sprintf("%s|%s|%s|%d", record.Host, record.Type, record.Value, record.Flag))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].Value < out[j].Value
	})
	return out
}
