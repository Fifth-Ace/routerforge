package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	lite "github.com/Fifth-Ace/routerforge/modules/dns-router-lite"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
	"github.com/miekg/dns"
)

var version = "dev"

func apiClient(socket string) *http.Client {
	return &http.Client{Timeout: 12 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirect forbidden") }}
}
func getJSON(client *http.Client, path string, dst any) error {
	r, e := client.Get("http://rf-dns/v1/" + path)
	if e != nil {
		return e
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return fmt.Errorf("API status %d", r.StatusCode)
	}
	d := json.NewDecoder(io.LimitReader(r.Body, 4<<20))
	if e = d.Decode(dst); e != nil {
		return e
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid API response")
	}
	return nil
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "STATE: FAIL /", e)
		os.Exit(1)
	}
}
func run() error {
	config := flag.String("config", "/opt/etc/routerforge/dns-router/config.json", "configuration")
	socket := flag.String("socket", "/opt/var/run/routerforge-dns-router.sock", "management socket")
	ui := flag.String("ui", "/opt/share/routerforge/modules/dns-router/ui", "UI directory")
	serve := flag.Bool("serve", false, "serve management; resume only explicitly enabled DNS")
	ver := flag.Bool("version", false, "version")
	evaluate := flag.String("evaluate", "", "offline policy evaluation")
	control := flag.String("service-control", "", "control independent DNS through its Unix socket")
	verify := flag.String("verify-dns", "", "verify direct or system DNS destination IP:port using per-query attribution")
	status := flag.Bool("service-status", false, "read service status through Unix socket")
	upgrade := flag.Bool("upgrade-precheck", false, "read-only check that no legacy/owned client DNS is connected before package upgrade")
	flag.Parse()
	if *ver {
		fmt.Println(version)
		return nil
	}
	if *control != "" || *verify != "" || *status || *upgrade {
		client := apiClient(*socket)
		var health struct {
			Component string `json:"component"`
		}
		if e := getJSON(client, "health", &health); e != nil {
			return e
		}
		if health.Component != "dns-router" {
			return errors.New("wrong API identity")
		}
		if *upgrade {
			var owner struct {
				State string `json:"state"`
			}
			if e := getJSON(client, "keenetic/managed-ownership/status", &owner); e != nil {
				return e
			}
			if owner.State != "disabled" {
				return fmt.Errorf("disable old P10 DNS before upgrade: %s", owner.State)
			}
			var s struct {
				Connection struct {
					State string `json:"state"`
				} `json:"connection"`
			}
			e := getJSON(client, "service/status", &s)
			if e == nil && s.Connection.State != "disconnected" {
				return errors.New("disconnect the owned test client before upgrade")
			}
			if e != nil && e.Error() != "API status 404" {
				return e
			}
			fmt.Println("UPGRADE_PRECHECK: PASS / no owned Keenetic DNS connection")
			return nil
		}
		if *status {
			var s map[string]any
			if e := getJSON(client, "service/status", &s); e != nil {
				return e
			}
			return json.NewEncoder(os.Stdout).Encode(s)
		}
		var stored struct {
			Config   dr.Config `json:"config"`
			Revision string    `json:"revision"`
		}
		if e := getJSON(client, "config", &stored); e != nil {
			return e
		}
		if *verify != "" {
			return verifyDNS(client, stored.Config, stored.Revision, *verify)
		}
		if *control != "start" && *control != "apply" && *control != "stop" {
			return errors.New("invalid service operation")
		}
		body, _ := json.Marshal(map[string]string{"operation": *control})
		req, _ := http.NewRequest("POST", "http://rf-dns/v1/service/control", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-RouterForge-Action", "service-control")
		req.Header.Set("If-Match", "\""+stored.Revision+"\"")
		r, e := client.Do(req)
		if e != nil {
			return e
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(r.Body, 4096))
		if r.StatusCode != 200 {
			return fmt.Errorf("service control: %s", b)
		}
		fmt.Println(string(b))
		return nil
	}
	store := dr.NewConfigStore(*config)
	c, _, e := store.Load()
	if e != nil {
		return e
	}
	if *evaluate != "" {
		engine, e := dr.Compile(c)
		if e != nil {
			return e
		}
		d, e := engine.EvaluateDNSQuestion(*evaluate)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(d)
	}
	if !*serve {
		fmt.Printf("CONFIG_PASS: %d rules; %d upstreams; no listeners opened\n", len(c.Rules), len(c.Upstreams))
		return nil
	}
	if os.Getenv("GOMEMLIMIT") == "" {
		debug.SetMemoryLimit(48 << 20)
	}
	manager := &lite.Manager{Store: store, Path: *config, Profile: &lite.ProfileManager{Path: *config + ".profile-state.json", Executor: dr.NewRealNDMCM()}, Gate: lite.NewGateManager()}
	if e = manager.Resume(); e != nil {
		log.Printf("DNS resume failed; management remains available: %v", e)
	}
	if e = manager.Profile.Reconcile(context.Background()); e != nil {
		log.Printf("DNS client connection needs inspection: %v", e)
	}
	defer manager.Close()
	defer manager.Gate.Close()
	if !filepath.IsAbs(*socket) {
		return errors.New("absolute Unix socket required")
	}
	if _, e = os.Lstat(*socket); e == nil {
		return errors.New("management socket already exists")
	} else if !os.IsNotExist(e) {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(*socket), 0700); e != nil {
		return e
	}
	mask := syscall.Umask(0077)
	listener, e := net.Listen("unix", *socket)
	syscall.Umask(mask)
	if e != nil {
		return e
	}
	defer listener.Close()
	defer os.Remove(*socket)
	if e = os.Chmod(*socket, 0600); e != nil {
		return e
	}
	api := dr.NewAPI(store, *ui)
	api.Service = manager
	api.LedgerO = dr.NewLedgerStoreO(*config + ".managed-state.json")
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go dr.RunManagedListUpdater(ctx, store, nil, log.Printf) // opt-in URL refresh updates the draft; runtime Apply stays explicit
	go func() {
		<-ctx.Done()
		s, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(s)
	}()
	e = server.Serve(listener)
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}

func verifyDNS(client *http.Client, c dr.Config, revision, destination string) error {
	policy, e := dr.Compile(c)
	if e != nil {
		return e
	}
	if c.Service == nil {
		return errors.New("independent DNS is not configured")
	}
	var status struct {
		Active   bool   `json:"active"`
		Revision string `json:"applied_revision"`
	}
	if e = getJSON(client, "service/status", &status); e != nil {
		return e
	}
	if !status.Active || status.Revision != revision {
		return errors.New("exact saved configuration is not active")
	}
	_, _, e = net.SplitHostPort(destination)
	if e != nil {
		return e
	}
	type sample struct{ name, assigned string }
	samples := []sample{}
	seen := map[string]bool{}
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return e
	}
	prefix := "rfdiag-" + hex.EncodeToString(nonce[:])
	for _, r := range c.Rules {
		if r.Match != "suffix" || seen[r.Upstream] {
			continue
		}
		name := prefix + "." + r.Domain
		d, e := policy.EvaluateDNSQuestion(name)
		if e == nil && d.Upstream == r.Upstream {
			samples = append(samples, sample{name, r.Upstream})
			seen[r.Upstream] = true
		}
	}
	defaultName := prefix + ".invalid"
	d, _ := policy.EvaluateDNSQuestion(defaultName)
	if d.Matched {
		return errors.New("diagnostic default name is covered by a rule")
	}
	samples = append(samples, sample{defaultName, c.Service.DefaultUpstream})
	fmt.Println("=== PRECHECK / DNS SERVICE E2E ===")
	fmt.Println("DESTINATION:", destination)
	fmt.Println("EXACT_REVISION:", revision)
	for _, s := range samples {
		for _, network := range []string{"udp", "tcp"} {
			name := network + "." + s.name
			q := new(dns.Msg)
			q.SetQuestion(name+".", dns.TypeA)
			a, _, er := (&dns.Client{Net: network, Timeout: 5 * time.Second}).Exchange(q, destination)
			if er != nil {
				return er
			}
			if a.Truncated || (a.Rcode != dns.RcodeSuccess && a.Rcode != dns.RcodeNameError) {
				return fmt.Errorf("DNS negative/failure response for %s", name)
			}
			var trace struct {
				Traces []lite.Trace `json:"traces"`
			}
			if er = getJSON(client, "service/traces?name="+name, &trace); er != nil {
				return er
			}
			proved := false
			for _, t := range trace.Traces {
				if t.Name == name && t.Type == dns.TypeA && t.Assigned == s.assigned && t.Used == s.assigned && t.Success && !t.Cache && t.RCode == a.Rcode {
					proved = true
				}
			}
			if !proved {
				return fmt.Errorf("ROUTE_NOT_PROVEN %s %s expected=%s", network, name, s.assigned)
			}
			fmt.Printf("ROUTE: PASS / %s / %s / %s\n", network, name, s.assigned)
		}
	}
	var after struct {
		Active   bool   `json:"active"`
		Revision string `json:"applied_revision"`
	}
	if e = getJSON(client, "service/status", &after); e != nil {
		return e
	}
	if !after.Active || after.Revision != revision {
		return errors.New("DNS state changed during verification")
	}
	fmt.Println("=== RESULT ===")
	fmt.Println("STATE: PASS / tested DNS paths and sampled upstreams")
	if strings.TrimSpace(destination) == c.Service.Listen {
		fmt.Println("SCOPE: DIRECT_SERVICE_ONLY / client profile and router:53 are not proven")
	}
	return nil
}
