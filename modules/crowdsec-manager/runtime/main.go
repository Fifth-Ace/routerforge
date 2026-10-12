package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/Fifth-Ace/routerforge/internal/safety"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var version = "dev"

const socketDefault = "/opt/var/run/routerforge-crowdsec-manager.sock"

type check struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}
type snapshot struct {
	Installed bool     `json:"installed"`
	Running   bool     `json:"running"`
	Protected bool     `json:"protected"`
	Firewall  check    `json:"firewall"`
	Sets      check    `json:"sets"`
	Hook      check    `json:"ndm_hook"`
	Config    check    `json:"config"`
	Chains    []string `json:"chains"`
	SetsNames []string `json:"set_names"`
	Notes     []string `json:"notes"`
	Updated   string   `json:"updated"`
}

func bounded(ctx context.Context, bin string, args ...string) (string, error) {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := safety.RunCommand(c, 64<<10, bin, args...)
	if len(out) > 64<<10 {
		out = out[:64<<10]
	}
	return string(out), err
}
func installed() bool { s, e := os.Stat("/opt/bin/cs-firewall-bouncer"); return e == nil && !s.IsDir() }
func running() bool {
	entries, e := os.ReadDir("/proc")
	if e != nil {
		return false
	}
	for _, ent := range entries {
		if _, e := strconvAtoi(ent.Name()); e != nil {
			continue
		}
		b, e := os.ReadFile("/proc/" + ent.Name() + "/cmdline")
		if e != nil {
			continue
		}
		cmd := strings.Split(string(b), "\x00")
		if len(cmd) > 0 && filepath.Base(cmd[0]) == "cs-firewall-bouncer" {
			return true
		}
	}
	return false
}
func strconvAtoi(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errors.New("nondigit")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
func snapshotNow(ctx context.Context) snapshot {
	s := snapshot{Installed: installed(), Running: running(), Updated: time.Now().UTC().Format(time.RFC3339), Chains: []string{}, SetsNames: []string{}, Notes: []string{}}
	_, e := os.Stat("/opt/etc/crowdsec/crowdsec-firewall-bouncer.yaml")
	s.Config = check{OK: e == nil, Detail: map[bool]string{true: "Файл конфигурации обнаружен; ключи не читаются", false: "Конфигурация не найдена"}[e == nil]}
	_, e = os.Stat("/opt/etc/ndm/netfilter.d/crowdsec.sh")
	s.Hook = check{OK: e == nil, Detail: map[bool]string{true: "NDM hook найден (работоспособность не подтверждена)", false: "NDM hook не найден — правила могут исчезнуть после перестройки firewall"}[e == nil]}
	names, err := bounded(ctx, "/opt/sbin/ipset", "list", "-n")
	if err != nil {
		names, err = bounded(ctx, "/opt/bin/ipset", "list", "-n")
	}
	if err == nil {
		for _, line := range strings.Split(names, "\n") {
			name := strings.TrimSpace(line)
			if strings.HasPrefix(name, "crowdsec-blacklists-") && len(s.SetsNames) < 100 {
				s.SetsNames = append(s.SetsNames, name)
			}
		}
	}
	s.Sets = check{OK: len(s.SetsNames) > 0, Detail: map[bool]string{true: "CrowdSec ipset найдены", false: "CrowdSec ipset не найдены"}[len(s.SetsNames) > 0]}
	// Do not infer banned IP counts from an ipset kernel format missing 'Number of entries'.
	output, err := bounded(ctx, "/usr/sbin/iptables", "-S", "FORWARD")
	if err != nil {
		output, err = bounded(ctx, "/opt/sbin/iptables", "-S", "FORWARD")
	}
	forward := err == nil && strings.Contains(output, "-j CROWDSEC_CHAIN")
	chain, chainErr := bounded(ctx, "/usr/sbin/iptables", "-S", "CROWDSEC_CHAIN")
	if chainErr != nil {
		chain, chainErr = bounded(ctx, "/opt/sbin/iptables", "-S", "CROWDSEC_CHAIN")
	}
	drop := chainErr == nil && strings.Contains(chain, "-j DROP") && strings.Contains(chain, "--match-set crowdsec-blacklists-")
	if forward {
		s.Chains = append(s.Chains, "FORWARD → CROWDSEC_CHAIN")
	}
	if drop {
		s.Chains = append(s.Chains, "CROWDSEC_CHAIN → ipset DROP")
	}
	s.Firewall = check{OK: forward && drop, Detail: map[bool]string{true: "FORWARD подключён к цепочке DROP ipset", false: "Не подтверждена связка FORWARD → CROWDSEC_CHAIN → ipset DROP"}[forward && drop]}
	s.Protected = s.Running && s.Sets.OK && s.Firewall.OK
	if !s.Running {
		s.Notes = append(s.Notes, "Bouncer не обнаружен среди запущенных процессов")
	}
	if !s.Hook.OK {
		s.Notes = append(s.Notes, "После сброса NDM защита может перестать фильтровать")
	}
	if s.Running && !s.Protected {
		s.Notes = append(s.Notes, "Процесс запущен, но действующие правила защиты не подтверждены")
	}
	return s
}
func respond(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(data)
}
func main() {
	sock := flag.String("socket", socketDefault, "Unix socket")
	ui := flag.String("ui", "/opt/share/routerforge/modules/crowdsec-manager/ui", "UI directory")
	flag.Parse()
	_ = os.MkdirAll(filepath.Dir(*sock), 0755)
	_ = os.Remove(*sock)
	lis, e := net.Listen("unix", *sock)
	if e != nil {
		panic(e)
	}
	defer lis.Close()
	defer os.Remove(*sock)
	if e = os.Chmod(*sock, 0600); e != nil {
		panic(e)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		respond(w, map[string]any{"ok": true, "module": "crowdsec-manager", "version": version, "mode": "bouncer-management"})
	})
	mux.HandleFunc("/v1/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		respond(w, snapshotNow(r.Context()))
	})
	registerManagementEndpoints(mux)
	fs := http.FileServer(http.Dir(*ui))
	mux.HandleFunc("/v1/ui/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		h := strings.TrimPrefix(r.URL.Path, "/v1/ui/")
		if h == "" {
			h = "index.html"
		}
		if filepath.IsAbs(h) || strings.Contains(h, "..") {
			http.NotFound(w, r)
			return
		}
		rr := r.Clone(r.Context())
		rr.URL.Path = "/" + h
		fs.ServeHTTP(w, rr)
	})
	server := http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	if e = server.Serve(lis); e != nil && !errors.Is(e, http.ErrServerClosed) {
		panic(e)
	}
}
