package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	dnsrouter "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

var version = "dev"

func main() {
	config := flag.String("config", "/opt/etc/routerforge/dns-router/config.json", "DNS Router config path")
	domain := flag.String("evaluate", "", "offline domain evaluation")
	showVersion := flag.Bool("version", false, "print version")
	hardwareReport := flag.Bool("hardware-report", false, "read-only local hardware evidence and integration design JSON; no configuration load")
	assessHardware := flag.String("assess-hardware-log", "", "offline assessment of a DPR2-G read-only evidence text file; no device access")
	assessHardwareH := flag.String("assess-h-report", "", "DPR2-H strictly read-only DSTest hardware evidence; no DNS activation")
	serve := flag.Bool("serve", false, "manual management-only Unix socket API")
	socket := flag.String("socket", "/opt/var/run/routerforge-dns-router.sock", "management socket")
	ui := flag.String("ui", "/opt/share/routerforge/modules/dns-router/ui", "UI directory")
	shadow := flag.Bool("shadow-forward", false, "EXPLICIT shadow UDP/TCP forwarder on loopback/high port; no router DNS takeover")
	listen := flag.String("shadow-listen", "127.0.0.1:55353", "shadow listener, never :53")
	ndmcTrial := flag.Bool("ndmc-transaction-experimental", false, "DSTest-only bounded ndmc trial with automatic exact rollback")
	managedTrial := flag.Bool("ndmc-managed-trial-experimental", false, "DSTest-only bounded 1..8 zone transactional trial with exact rollback")
	managedWorker := flag.String("ndmc-managed-rollback-worker", "", "internal multi-zone watchdog process")
	durableWorker := flag.String("ndmc-durable-disable-worker", "", "internal reverse rollback watchdog")
	durableOptIn := flag.Bool("ndmc-durable-experimental", false, "DSTest-only explicit persistent zone API; NEVER automatic startup")
	rollbackWorker := flag.String("ndmc-rollback-worker", "", "internal independent rollback watchdog")
	allowLAN := flag.Bool("shadow-lan-experimental", false, "EXPLICIT private IPv4 LAN test listener; requires source ACL")
	clients := flag.String("shadow-allow-clients", "", "comma-separated exact IPv4 source allowlist for LAN shadow")
	flag.Parse()
	if *durableWorker != "" {
		if flag.NArg() != 0 {
			log.Fatal("durable watchdog arguments forbidden")
		}
		if err := dnsrouter.DurableDisableWorkerO2(*durableWorker); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *managedWorker != "" {
		if flag.NArg() != 0 {
			log.Fatal("managed watchdog arguments are forbidden")
		}
		if err := dnsrouter.ManagedWorkerN(*managedWorker); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *rollbackWorker != "" {
		if flag.NArg() != 0 {
			log.Fatal("watchdog arguments are forbidden")
		}
		if err := dnsrouter.NDMCWatchdogWorkerM(*rollbackWorker); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *durableOptIn && (!*serve || !*allowLAN || !*shadow) {
		log.Fatal("durable experiment requires --serve --shadow-lan-experimental --shadow-forward")
	}
	if *managedTrial && (!*serve || !*allowLAN) {
		log.Fatal("managed zone trial requires --serve --shadow-lan-experimental")
	}
	if *ndmcTrial && (!*serve || !*allowLAN) {
		log.Fatal("ndmc trial requires --serve --shadow-lan-experimental")
	}
	if *showVersion {
		fmt.Println(version)
		return
	}
	if (*allowLAN || *clients != "") && !*serve {
		log.Fatal("LAN capability requires --serve")
	}
	if *clients != "" && !*allowLAN {
		log.Fatal("LAN client ACL requires --shadow-lan-experimental")
	}
	if *shadow && !*serve {
		log.Fatal("--shadow-forward requires --serve")
	}
	if *assessHardwareH != "" {
		if *assessHardware != "" || *serve || *shadow || *domain != "" || *hardwareReport {
			log.Fatal("--assess-h-report is offline-only and cannot be combined with runtime operations")
		}
		data, err := os.ReadFile(*assessHardwareH)
		if err != nil {
			log.Fatal(err)
		}
		result, err := dnsrouter.ParseHardwareHReadiness(string(data))
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *assessHardware != "" {
		if *serve || *shadow || *domain != "" || *hardwareReport {
			log.Fatal("--assess-hardware-log cannot be combined with serving, evaluation or other hardware operations")
		}
		file, err := os.Open(*assessHardware)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		limited := io.LimitReader(file, 16<<10+1)
		raw, err := io.ReadAll(limited)
		if err != nil {
			log.Fatal(err)
		}
		result, err := dnsrouter.ParseHardwareReadOnlyEvidence(string(raw))
		if err != nil {
			log.Fatal(err)
		}
		if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *hardwareReport {
		if *serve || *shadow || *domain != "" {
			log.Fatal("--hardware-report cannot be combined with serving or evaluation")
		}
		d := dnsrouter.ReadOnlyKeeneticDiscovery()
		report := map[string]any{
			"mode": "read-only", "discovery": d,
			"resources":        dnsrouter.CurrentResourceSnapshot(),
			"integration_plan": dnsrouter.BuildKeeneticIntegrationPlan(d),
			"changes_applied":  false, "hardware_accepted": false,
		}
		if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
			log.Fatal(err)
		}
		return
	}
	store := dnsrouter.NewConfigStore(*config)
	if *serve {
		if err := runManagement(store, *config, *socket, *ui, *shadow, *listen, *allowLAN, *clients, *ndmcTrial, *managedTrial, *durableOptIn); err != nil {
			log.Fatal(err)
		}
		return
	}
	engine, err := dnsrouter.Load(*config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CONFIG_FAIL:", err)
		os.Exit(1)
	}
	if *domain == "" {
		fmt.Printf("CONFIG_PASS: %d rules; %d upstream pools; inactive, no sockets opened\n", engine.RuleCount(), len(engine.UpstreamIDs()))
		return
	}
	d, err := engine.Evaluate(*domain)
	if err != nil {
		fmt.Fprintln(os.Stderr, "EVALUATION_FAIL:", err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(d)
}

func runManagement(store *dnsrouter.ConfigStore, configPath, socket, ui string, enableShadow bool, shadowListen string, allowLAN bool, clients string, ndmcTrial bool, managedTrial bool, durableOptIn bool) error {
	if !filepath.IsAbs(socket) {
		return fmt.Errorf("Unix socket path must be absolute")
	}
	cfg, _, err := store.Load()
	if err != nil {
		return fmt.Errorf("configuration precheck: %w", err)
	}
	_ = cfg
	var allowed []string
	if clients != "" {
		allowed = strings.Split(clients, ",")
	}
	lifecycle := dnsrouter.NewLifecycleManager(store, dnsrouter.ShadowOptions{Listen: shadowListen, AllowLAN: allowLAN, AllowedClients: allowed})
	if enableShadow {
		_, rev, err := store.Load()
		if err != nil {
			return err
		}
		if err := lifecycle.Start(rev); err != nil {
			return fmt.Errorf("manual shadow startup: %w", err)
		}
	}
	defer lifecycle.Close()
	if _, err := os.Lstat(socket); err == nil {
		return fmt.Errorf("socket path exists; refusing overwrite: %s", socket)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0700); err != nil {
		return err
	}
	mask := syscall.Umask(0077)
	listener, err := net.Listen("unix", socket)
	syscall.Umask(mask)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socket)
	if err := os.Chmod(socket, 0600); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	handler := dnsrouter.NewAPI(store, ui)
	handler.Lifecycle = lifecycle
	handler.LedgerO = dnsrouter.NewLedgerStoreO(configPath + ".managed-state.json")
	if ndmcTrial {
		if err := dnsrouter.NDMCTestHostGateM(); err != nil {
			return err
		}
		handler.TrialM = &dnsrouter.NDMCTrialM{Store: store, Lifecycle: lifecycle, Executor: dnsrouter.NewRealNDMCM(), Probe: dnsrouter.NewRealProbeM(), Arm: dnsrouter.ArmOSGuardM}
	}
	if managedTrial {
		if err := dnsrouter.NDMCTestHostGateM(); err != nil {
			return err
		}
		handler.ManagedN = &dnsrouter.ManagedTrialN{Store: store, Lifecycle: lifecycle, Executor: dnsrouter.NewRealNDMCM(), Arm: dnsrouter.ArmOSGuardN, Probe: dnsrouter.ProbeManagedN(lifecycle)}
	}
	if durableOptIn {
		if err := dnsrouter.NDMCTestHostGateM(); err != nil {
			return err
		}
		handler.DurableO2 = dnsrouter.NewRealDurableO2(store, handler.LedgerO, lifecycle)
	}
	server := &http.Server{Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second}

	httpDone := make(chan error, 1)
	go func() { httpDone <- server.Serve(listener) }()
	log.Printf("DNS Router manual management socket %s; shadow_initial=%t; NO :53/interception/Keenetic changes", socket, enableShadow)
	var cause error
	select {
	case <-ctx.Done():
	case cause = <-httpDone:
		if cause == http.ErrServerClosed {
			cause = nil
		}

	}
	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	// Cancel terminates both UDP and TCP listeners and workers.
	return cause
}
