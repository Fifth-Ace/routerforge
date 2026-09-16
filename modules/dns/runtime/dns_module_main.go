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
	policyEgressSmoke := flag.String("policy-egress-smoke", "", "run one marked-egress diagnostic for PolicyN and exit")
	policyEgressSmokeAddress := flag.String("policy-egress-smoke-address", "1.1.1.1:443", "TCP destination for marked-egress diagnostic")
	policyEgressSmokeTimeout := flag.Duration("policy-egress-smoke-timeout", 4*time.Second, "timeout for marked-egress diagnostic")
	policyShadowSmoke := flag.Bool("policy-shadow-smoke", false, "run loopback-only shadow DNS forwarding smoke and exit")
	policyShadowSmokeListen := flag.String("policy-shadow-smoke-listen", "127.0.0.1:55353", "loopback non-53 listener for shadow DNS smoke")
	policyShadowSmokeUpstream := flag.String("policy-shadow-smoke-upstream", "1.1.1.1:53", "explicit IP:port upstream for shadow DNS smoke")
	policyShadowSmokeTimeout := flag.Duration("policy-shadow-smoke-timeout", 3*time.Second, "per-query timeout for shadow DNS smoke")
	flag.Parse()

	if os.Geteuid() != 0 {
		log.Fatal("routerforge-dns must run as root (packet capture and Keenetic RCI require it)")
	}

	if *policyEgressSmoke != "" {
		result, err := runDNSPolicyEgressSmoke(*policyEgressSmoke, *policyEgressSmokeAddress, *policyEgressSmokeTimeout)
		fmt.Println("=== POLICY EGRESS SMOKE ===")
		fmt.Printf("POLICY: %s\n", result.Policy)
		fmt.Printf("ADDRESS: %s\n", result.Address)
		fmt.Printf("REQUESTED_MARK: %#x\n", result.RequestedMark)
		fmt.Printf("ACTUAL_MARK: %#x\n", result.ActualMark)
		fmt.Printf("TABLE: %d\n", result.Table)
		fmt.Printf("HAS_DEFAULT: %t\n", result.HasDefault)
		fmt.Printf("CONNECTED: %t\n", result.Connected)
		if err != nil {
			fmt.Printf("RESULT: FAIL\n")
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(3)
		}
		fmt.Println("RESULT: PASS")
		return
	}

	if *policyShadowSmoke {
		result, err := runDNSPolicyShadowSmoke(*policyShadowSmokeListen, *policyShadowSmokeUpstream, *policyShadowSmokeTimeout)
		fmt.Println("=== POLICY SHADOW SMOKE ===")
		fmt.Printf("LISTEN: %s\n", result.ListenAddr)
		fmt.Printf("UPSTREAM: %s\n", result.Upstream)
		for _, item := range result.Cases {
			fmt.Printf("CASE: %s TRANSPORT=%s POLICY=%s WANT_MARK=%#x REPLY=%t", item.Name, item.Transport, item.Policy, item.WantMark, item.Reply)
			if item.Error != "" {
				fmt.Printf(" ERROR=%q", item.Error)
			}
			fmt.Println()
		}
		fmt.Printf("POLICY1_MARK_COUNT: %d\n", countDNSPolicyShadowMark(result.Marks, 0x0ffffaab))
		fmt.Printf("POLICY0_MARK_COUNT: %d\n", countDNSPolicyShadowMark(result.Marks, 0x0ffffaaa))
		if err != nil {
			fmt.Println("RESULT: FAIL")
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(4)
		}
		fmt.Println("RESULT: PASS")
		return
	}

	store := NewStore(defaultFlowRetentionCap, 500)
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
