package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	bouncerConfigPath = "/opt/etc/crowdsec/crowdsec-firewall-bouncer.yaml"
	bouncerInitPath   = "/opt/etc/init.d/S99csfirewall"
	ndmHookPath       = "/opt/etc/ndm/netfilter.d/crowdsec.sh"
	managerAuthHeader = "X-RouterForge-Module-Authorized"
	managerAuthValue  = "core-authorized-v1"
)

var managementMu sync.Mutex
var metricNameRe = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*(\{[^\n]{0,300}\})?$`)

// No automatic firewall writes: NDM executes this hook only after its own rebuild.
// Adapted from siamskoi/keenetic-crowdsec-bouncer (see upstream README section 9).
const managedNDMHook = `#!/bin/sh
# RouterForge managed hook; adapted from siamskoi/keenetic-crowdsec-bouncer.
# Installed only after explicit user confirmation. No direct invocation from the UI.
PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin
[ "${type:-}" = "iptables" ] || exit 0
[ "${table:-}" = "filter" ] || exit 0
IPSET="$(command -v ipset)" || exit 0
IPTABLES="$(command -v iptables)" || exit 0
SETS="$($IPSET list -n 2>/dev/null | grep '^crowdsec-blacklists-' || true)"
[ -n "$SETS" ] || exit 0
$IPTABLES -n -L CROWDSEC_CHAIN >/dev/null 2>&1 || $IPTABLES -N CROWDSEC_CHAIN || exit 1
$IPTABLES -F CROWDSEC_CHAIN || exit 1
for setname in $SETS; do
    $IPTABLES -A CROWDSEC_CHAIN -m set --match-set "$setname" src -j DROP || exit 1
done
$IPTABLES -C FORWARD -j CROWDSEC_CHAIN >/dev/null 2>&1 || $IPTABLES -I FORWARD 1 -j CROWDSEC_CHAIN
exit $?
`

type configSummary struct {
	Exists         bool   `json:"exists"`
	URL            string `json:"api_url,omitempty"`
	KeySet         bool   `json:"api_key_set"`
	TLSVerify      bool   `json:"tls_verify"`
	MetricsEnabled bool   `json:"metrics_enabled"`
	MetricsPort    int    `json:"metrics_port,omitempty"`
	Error          string `json:"error,omitempty"`
}

type setSummary struct {
	Name       string   `json:"name"`
	Members    int      `json:"members"`
	CountKnown bool     `json:"count_known"`
	Samples    []string `json:"samples,omitempty"`
}

type managerOverview struct {
	Snapshot       snapshot           `json:"snapshot"`
	State          string             `json:"state"`
	Config         configSummary      `json:"lapi"`
	InitInstalled  bool               `json:"init_installed"`
	HookExecutable bool               `json:"hook_executable"`
	HookManaged    bool               `json:"hook_managed"`
	SetDetails     []setSummary       `json:"set_details"`
	Metrics        map[string]float64 `json:"metrics"`
	MetricsError   string             `json:"metrics_error,omitempty"`
	LAPIProbe      string             `json:"lapi_probe"`
	DropCounters   string             `json:"drop_counters,omitempty"`
}

// Reads only first-level YAML scalars and prometheus child settings; never returns api_key.
func readConfig() (configSummary, []byte) {
	info, err := os.Lstat(bouncerConfigPath)
	if err != nil {
		return configSummary{TLSVerify: true}, nil
	}
	if !info.Mode().IsRegular() || info.Size() > 128<<10 {
		return configSummary{Exists: true, TLSVerify: true, Error: "invalid config file type or size"}, nil
	}
	data, err := os.ReadFile(bouncerConfigPath)
	if err != nil {
		return configSummary{Exists: true, TLSVerify: true, Error: "config unreadable"}, nil
	}
	c := configSummary{Exists: true, TLSVerify: true}
	prometheus := false
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		key, value, ok := strings.Cut(trim, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(strings.SplitN(value, " #", 2)[0])
		value = strings.Trim(value, `"'`)
		if indent == 0 {
			prometheus = key == "prometheus"
		}
		if indent == 0 {
			switch key {
			case "api_url":
				c.URL = value
			case "api_key":
				c.KeySet = value != ""
			case "insecure_skip_verify":
				c.TLSVerify = value != "true"
			}
		} else if prometheus {
			switch key {
			case "enabled":
				c.MetricsEnabled = value == "true"
			case "listen_port":
				c.MetricsPort, _ = strconv.Atoi(value)
			}
		}
	}
	if c.MetricsPort <= 0 || c.MetricsPort > 65535 {
		c.MetricsPort = 60601
	}
	return c, data
}

func safeSetDetails(ctx context.Context, names []string) []setSummary {
	result := []setSummary{}
	for _, name := range names {
		if len(result) >= 20 {
			break
		}
		if !regexp.MustCompile(`^crowdsec-blacklists-[a-zA-Z0-9_-]+$`).MatchString(name) {
			continue
		}
		out, err := bounded(ctx, "/opt/sbin/ipset", "list", name)
		if err != nil {
			out, err = bounded(ctx, "/opt/bin/ipset", "list", name)
		}
		item := setSummary{Name: name, Samples: []string{}}
		if err == nil {
			truncated := len(out) >= 64<<10
			members := false
			numericCount := false
			for _, line := range strings.Split(out, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Number of entries:") {
					n, e := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Number of entries:")))
					if e == nil {
						item.Members = n
						item.CountKnown = true
						numericCount = true
					}
				}
				if line == "Members:" {
					members = true
					continue
				}
				if members && line != "" && !strings.HasPrefix(line, "#") {
					// Keenetic's ipset protocol may omit Number of entries entirely.
					if !item.CountKnown {
						item.Members++
					}
					if len(item.Samples) < 10 {
						item.Samples = append(item.Samples, strings.Fields(line)[0])
					}
				}
			}
			if members {
				item.CountKnown = !truncated || numericCount
			}
		}
		result = append(result, item)
	}
	return result
}

func readMetrics(c configSummary) (map[string]float64, string) {
	result := map[string]float64{}
	if !c.MetricsEnabled {
		return result, "metrics disabled"
	}
	// Always dial loopback; configured listen address is never trusted as a destination.
	client := http.Client{Timeout: 1300 * time.Millisecond, Transport: &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 900 * time.Millisecond}).DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(c.MetricsPort)))
	}}}
	resp, err := client.Get("http://127.0.0.1/metrics")
	if err != nil {
		return result, "local metrics endpoint unreachable"
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return result, fmt.Sprintf("metrics HTTP %d", resp.StatusCode)
	}
	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 128<<10))
	scanner.Buffer(make([]byte, 4096), 32<<10)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == '#' {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 || !metricNameRe.MatchString(parts[0]) {
			continue
		}
		name := strings.SplitN(parts[0], "{", 2)[0]
		if name == "fw_bouncer_banned_ips" {
			// Known false-zero on default Keenetic bouncer builds.
			continue
		}
		if !(strings.HasPrefix(name, "fw_bouncer_") || strings.HasPrefix(name, "lapi_requests_")) {
			continue
		}
		v, e := strconv.ParseFloat(parts[1], 64)
		if e == nil && len(result) < 75 {
			result[parts[0]] = v
		}
	}
	if scanner.Err() != nil {
		return result, "metrics truncated or unreadable"
	}
	return result, ""
}

// The health probe never transmits the API key; it only verifies HTTPS/TCP
// reachability. Bouncer authorization is best assessed from its LAPI metrics.
func probeLAPI(ctx context.Context, c configSummary) string {
	if c.URL == "" {
		return "LAPI URL not configured"
	}
	if err := safeURL(c.URL); err != nil {
		return "invalid LAPI URL"
	}
	probeCtx, cancel := context.WithTimeout(ctx, 1700*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, strings.TrimRight(c.URL, "/")+"/health", nil)
	if err != nil {
		return "invalid health request"
	}
	client := http.Client{Timeout: 1700 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "health endpoint unreachable or TLS unverified"
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return "LAPI health endpoint reachable (API key not validated)"
	}
	return fmt.Sprintf("LAPI health HTTP %d (API key not validated)", resp.StatusCode)
}

func getOverview(ctx context.Context) managerOverview {
	s := snapshotNow(ctx)
	conf, _ := readConfig()
	h, err := os.Lstat(ndmHookPath)
	hookExec := err == nil && h.Mode().IsRegular() && h.Mode()&0111 != 0
	hookData, _ := os.ReadFile(ndmHookPath)
	managed := hookExec && sha256.Sum256(hookData) == sha256.Sum256([]byte(managedNDMHook))
	init, err := os.Stat(bouncerInitPath)
	initOK := err == nil && init.Mode().IsRegular() && init.Mode()&0111 != 0
	sets := safeSetDetails(ctx, s.SetsNames)
	metrics, metricErr := readMetrics(conf)
	probe := probeLAPI(ctx, conf)
	state := "healthy"
	if !s.Installed || !initOK {
		state = "not_installed"
	} else if !s.Running || !s.Firewall.OK || !s.Sets.OK || !hookExec || !conf.Exists || !conf.KeySet || conf.URL == "" || !conf.TLSVerify || strings.HasPrefix(conf.URL, "http://") || !strings.HasPrefix(probe, "LAPI health endpoint reachable") {
		state = "degraded"
	}
	if conf.Error != "" {
		state = "error"
	}
	counters, _ := bounded(ctx, "/usr/sbin/iptables", "-L", "CROWDSEC_CHAIN", "-n", "-v", "-x")
	if counters == "" {
		counters, _ = bounded(ctx, "/opt/sbin/iptables", "-L", "CROWDSEC_CHAIN", "-n", "-v", "-x")
	}
	if len(counters) > 2500 {
		counters = counters[:2500]
	}
	return managerOverview{Snapshot: s, State: state, Config: conf, InitInstalled: initOK, HookExecutable: hookExec, HookManaged: managed, SetDetails: sets, Metrics: metrics, MetricsError: metricErr, LAPIProbe: probe, DropCounters: counters}
}

func writeManager(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func mutation(w http.ResponseWriter, r *http.Request, fn func(map[string]string) error) {
	if r.Method != http.MethodPost {
		writeManager(w, 405, map[string]string{"error": "POST required"})
		return
	}
	if r.Header.Get(managerAuthHeader) != managerAuthValue {
		writeManager(w, 403, map[string]string{"error": "Core authorization required"})
		return
	}
	defer r.Body.Close()
	var req map[string]string
	dec := json.NewDecoder(io.LimitReader(r.Body, 8193))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || len(req) > 7 || r.ContentLength > 8192 {
		writeManager(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	managementMu.Lock()
	defer managementMu.Unlock()
	if err := fn(req); err != nil {
		writeManager(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeManager(w, 200, map[string]bool{"ok": true})
}

func bouncerService(ctx context.Context, req map[string]string) error {
	action := req["action"]
	if req["confirm"] != "crowdsec-service" || (action != "start" && action != "stop" && action != "restart") {
		return errors.New("explicit service confirmation required")
	}
	info, err := os.Lstat(bouncerInitPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return errors.New("known bouncer init script is missing")
	}
	c, cancel := context.WithTimeout(ctx, 70*time.Second)
	defer cancel()
	output, err := safety.RunCommand(c, 16<<10, bouncerInitPath, action)
	_ = output // Never reflect script output; it can contain credentials.
	if err != nil {
		return errors.New("bouncer init action failed; inspect device logs")
	}
	return nil
}

func safeURL(value string) error {
	if len(value) > 400 || strings.ContainsAny(value, "\r\n\t ") {
		return errors.New("invalid LAPI URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" {
		return errors.New("invalid LAPI URL")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return errors.New("LAPI URL requires http or https")
	}
	return nil
}

func updateLAPI(req map[string]string) error {
	if req["confirm"] != "crowdsec-lapi" {
		return errors.New("explicit LAPI save confirmation required")
	}
	c, old := readConfig()
	if !c.Exists || c.Error != "" || old == nil {
		return errors.New("existing bouncer config required")
	}
	target := req["api_url"]
	if err := safeURL(target); err != nil {
		return err
	}
	key := req["api_key"]
	if strings.ContainsAny(key, "\r\n\x00") || len(key) > 2048 {
		return errors.New("invalid API key")
	}
	if key == "" && !c.KeySet {
		return errors.New("API key is required for initial setup")
	}
	lines := strings.Split(string(old), "\n")
	found := map[string]bool{}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		if len(line) != len(strings.TrimLeft(line, " \t")) {
			continue
		}
		k, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "api_url" || (k == "api_key" && key != "") {
			if found[k] {
				return errors.New("duplicate LAPI config key")
			}
			found[k] = true
			if k == "api_url" {
				lines[i] = "api_url: " + strconv.Quote(target)
			} else {
				lines[i] = "api_key: " + strconv.Quote(key)
			}
		}
	}
	if !found["api_url"] {
		lines = append(lines, "api_url: "+strconv.Quote(target))
	}
	if key != "" && !found["api_key"] {
		lines = append(lines, "api_key: "+strconv.Quote(key))
	}
	dir := filepath.Dir(bouncerConfigPath)
	if dir != "/opt/etc/crowdsec" {
		return errors.New("invalid config directory")
	}
	if err := safety.WriteFileAtomic(bouncerConfigPath+".routerforge.bak", old, 0600); err != nil {
		return errors.New("failed to back up bouncer config")
	}
	if err := safety.WriteFileAtomic(bouncerConfigPath, []byte(strings.Join(lines, "\n")), 0600); err != nil {
		return errors.New("failed to write bouncer config")
	}
	return nil
}

func manageHook(req map[string]string) error {
	if req["confirm"] != "crowdsec-hook" {
		return errors.New("explicit NDM hook confirmation required")
	}
	action := req["action"]
	current, err := os.Lstat(ndmHookPath)
	if err == nil {
		if !current.Mode().IsRegular() {
			return errors.New("existing NDM hook is not a regular file")
		}
		body, readErr := os.ReadFile(ndmHookPath)
		if readErr != nil {
			return errors.New("unable to inspect existing NDM hook")
		}
		if sha256.Sum256(body) != sha256.Sum256([]byte(managedNDMHook)) {
			return errors.New("unmanaged NDM hook exists; refusing to overwrite or remove")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("unable to inspect NDM hook")
	}
	switch action {
	case "install":
		if err := os.MkdirAll(filepath.Dir(ndmHookPath), 0755); err != nil {
			return errors.New("unable to create NDM directory")
		}
		if err := safety.WriteFileAtomic(ndmHookPath, []byte(managedNDMHook), 0755); err != nil {
			return errors.New("unable to install NDM hook")
		}
	case "remove":
		if err != nil {
			return errors.New("managed NDM hook does not exist")
		}
		if err := os.Remove(ndmHookPath); err != nil {
			return errors.New("unable to remove managed hook")
		}
	default:
		return errors.New("unsupported hook action")
	}
	return nil
}

func registerManagementEndpoints(mux *http.ServeMux) {
	mux.HandleFunc("/v1/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		writeManager(w, 200, getOverview(r.Context()))
	})
	mux.HandleFunc("/v1/service", func(w http.ResponseWriter, r *http.Request) {
		mutation(w, r, func(req map[string]string) error { return bouncerService(r.Context(), req) })
	})
	mux.HandleFunc("/v1/lapi", func(w http.ResponseWriter, r *http.Request) { mutation(w, r, updateLAPI) })
	mux.HandleFunc("/v1/hook", func(w http.ResponseWriter, r *http.Request) { mutation(w, r, manageHook) })
}
