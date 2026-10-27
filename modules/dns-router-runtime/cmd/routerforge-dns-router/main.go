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
	config := flag.String("config", "/opt/etc/routerforge/dns-router/config.json", "path to v1 DNS Router config")
	domain := flag.String("evaluate", "", "offline evaluation of one DNS name")
	showVersion := flag.Bool("version", false, "print build version")
	serve := flag.Bool("serve", false, "enable manual management-only Unix socket API (NO DNS listener)")
	socket := flag.String("socket", "/opt/var/run/routerforge-dns-router.sock", "Unix socket path")
	ui := flag.String("ui", "/opt/share/routerforge/modules/dns-router/ui", "UI directory")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	store := dnsrouter.NewConfigStore(*config)
	if *serve {
		if err := runManagement(store, *socket, *ui); err != nil {
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
func runManagement(store *dnsrouter.ConfigStore, socket, ui string) error {
	if !filepath.IsAbs(socket) {
		return fmt.Errorf("Unix socket path must be absolute")
	}
	if _, _, err := store.Load(); err != nil {
		return fmt.Errorf("configuration precheck: %w", err)
	}
	if _, err := os.Lstat(socket); err == nil {
		return fmt.Errorf("socket path exists; refusing overwrite: %s", socket)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(socket), 0700); err != nil {
		return err
	}
	old := syscall.Umask(0077)
	listener, err := net.Listen("unix", socket)
	syscall.Umask(old)
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(socket)
	if err := os.Chmod(socket, 0600); err != nil {
		return err
	}
	server := &http.Server{Handler: dnsrouter.NewAPI(store, ui).Handler(), ReadHeaderTimeout: 5 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(stop)
	done := make(chan struct{})
	go func() {
		select {
		case <-stop:
		case <-done:
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()
	log.Printf("DNS Router management API on %s (NO DNS forwarding or interception)", socket)
	err = server.Serve(listener)
	close(done)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
