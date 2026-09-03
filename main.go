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
	listen := flag.String("listen", ":2233", "web listen address")
	discoveryEvery := flag.Duration("discovery", 60*time.Second, "Keenetic DNS config refresh interval")
	healthEvery := flag.Duration("health", 30*time.Second, "resolver health check interval")
	logPath := flag.String("log", "/opt/var/log/dns-monitor.log", "event log path")
	flag.Parse()

	if os.Geteuid() != 0 {
		log.Fatal("dns-monitor must run as root (packet capture requires it)")
	}

	store := NewStore(10000, 500)
	eventLog := NewEventLogger(*logPath)
	eventLog.Event("START", fmt.Sprintf("dns-monitor v%s listen=%s", version, *listen))

	startOptionalProfiling()

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

	fmt.Printf("dns-monitor v%s\n", version)
	fmt.Printf("web listen: %s\n", *listen)
	fmt.Printf("event log: %s\n", *logPath)

	if err := startWeb(store, *listen, version); err != nil {
		eventLog.Event("WEB_ERROR", err.Error())
		log.Fatal(err)
	}
}
