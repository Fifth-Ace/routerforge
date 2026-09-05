//go:build routerforge_dns_module

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var version = "dev"

func main() {
	socket := flag.String("socket", "/opt/var/run/routerforge-dns.sock", "Unix socket path")
	discoveryEvery := flag.Duration("discovery", 60*time.Second, "Keenetic DNS config refresh interval")
	healthEvery := flag.Duration("health", 30*time.Second, "resolver health check interval")
	logPath := flag.String("log", "/opt/var/log/routerforge-dns.log", "DNS event log path")
	uiPath := flag.String("ui", "/opt/share/routerforge/modules/dns/ui", "DNS module UI directory")
	flag.Parse()

	if os.Geteuid() != 0 {
		log.Fatal("routerforge-dns must run as root (packet capture and Keenetic RCI require it)")
	}

	store := NewStore(10000, 500)
	eventLog := NewEventLogger(*logPath)
	eventLog.Event("START", fmt.Sprintf("routerforge-dns v%s socket=%s", version, *socket))

	go discoveryLoop(store, *discoveryEvery, eventLog)
	go clientRegistryLoop(store, eventLog)
	go healthLoop(store, *healthEvery, eventLog)
	go diagnosticLoop(store, eventLog)

	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for now := range t.C {
			store.CleanupTransient(now, eventLog)
			plainDNS.Sweep(now)
			eventLog.FlushDNSFailures()
		}
	}()

	go func() {
		time.Sleep(1200 * time.Millisecond)
		if err := captureLoop(store, eventLog); err != nil {
			store.SetCaptureError(err.Error())
			eventLog.Event("CAPTURE_ERROR", err.Error())
			log.Printf("capture disabled: %v", err)
		}
	}()

	go func() {
		time.Sleep(1500 * time.Millisecond)
		if err := clientCaptureLoop(store, eventLog); err != nil {
			store.SetClientCaptureError(err.Error())
			eventLog.Event("CLIENT_CAPTURE_ERROR", err.Error())
			log.Printf("client capture disabled: %v", err)
		}
	}()

	fmt.Printf("RouterForge DNS v%s\n", version)
	fmt.Printf("socket: %s\n", *socket)
	fmt.Printf("UI: %s\n", *uiPath)

	server := newDNSModuleServer(store, version, *socket, *uiPath)
	if err := server.Serve(); err != nil {
		eventLog.Event("SERVER_ERROR", err.Error())
		log.Fatal(err)
	}
}
