package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	dnsrouter "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

var version = "dev"

func main() {
	config := flag.String("config", "/opt/etc/routerforge/dns-router/config.json", "DNS Router config path")
	domain := flag.String("evaluate", "", "offline domain evaluation")
	showVersion := flag.Bool("version", false, "print version")
	serve := flag.Bool("serve", false, "manual management-only Unix socket API")
	socket := flag.String("socket", "/opt/var/run/routerforge-dns-router.sock", "management socket")
	ui := flag.String("ui", "/opt/share/routerforge/modules/dns-router/ui", "UI directory")
	shadow := flag.Bool("shadow-forward", false, "EXPLICIT shadow UDP/TCP forwarder on loopback/high port; no router DNS takeover")
	listen := flag.String("shadow-listen", "127.0.0.1:55353", "shadow loopback listener, never :53")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *shadow && !*serve {
		log.Fatal("--shadow-forward requires --serve")
	}
	store := dnsrouter.NewConfigStore(*config)
	if *serve {
		if err := runManagement(store, *socket, *ui, *shadow, *listen); err != nil {
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

func runManagement(store *dnsrouter.ConfigStore, socket, ui string, enableShadow bool, shadowListen string) error {
	if !filepath.IsAbs(socket) {
		return fmt.Errorf("Unix socket path must be absolute")
	}
	cfg, _, err := store.Load()
	if err != nil {
		return fmt.Errorf("configuration precheck: %w", err)
	}
	var forward *dnsrouter.ShadowForwarder
	if enableShadow {
		forward, err = dnsrouter.NewShadowForwarder(cfg, dnsrouter.ShadowOptions{Listen: shadowListen})
		if err != nil {
			return fmt.Errorf("shadow precheck: %w", err)
		}
	}
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
	handler.Shadow = forward
	server := &http.Server{Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second}
	runError := make(chan error, 1)
	if forward != nil {
		go func() { runError <- forward.Serve(ctx) }()
	}
	httpDone := make(chan error, 1)
	go func() { httpDone <- server.Serve(listener) }()
	log.Printf("DNS Router manual management socket %s; shadow=%t; NO :53/interception/Keenetic changes", socket, enableShadow)
	var cause error
	select {
	case <-ctx.Done():
	case cause = <-httpDone:
		if cause == http.ErrServerClosed {
			cause = nil
		}
	case cause = <-runError:
		// A shadow listener error must stop the management API too.
		if cause == nil && ctx.Err() == nil {
			cause = fmt.Errorf("shadow stopped unexpectedly")
		}
	}
	stop()
	shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdown)
	// Cancel terminates both UDP and TCP listeners and workers.
	return cause
}
