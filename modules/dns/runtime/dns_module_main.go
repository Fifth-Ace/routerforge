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
	policyPersistedShadow := flag.Bool("policy-persisted-shadow", false, "run controlled persisted-rule shadow listener and exit")
	policyPersistedShadowStore := flag.String("policy-persisted-shadow-store", "/opt/etc/routerforge/dns-policy-rules.json", "persisted policy document path")
	policyPersistedShadowListen := flag.String("policy-persisted-shadow-listen", "127.0.0.1:55354", "loopback non-53 listener for persisted shadow mode")
	policyPersistedShadowUpstream := flag.String("policy-persisted-shadow-upstream", "1.1.1.1:53", "explicit IP:port upstream for persisted shadow mode")
	policyPersistedShadowDuration := flag.Duration("policy-persisted-shadow-duration", 15*time.Second, "bounded persisted shadow listener lifetime")
	policyPersistedShadowTimeout := flag.Duration("policy-persisted-shadow-timeout", 3*time.Second, "per-query persisted shadow timeout")
	policyPersistedShadowConcurrency := flag.Int("policy-persisted-shadow-concurrency", 16, "bounded persisted shadow concurrency")
	policyIngressSmoke := flag.Bool("policy-ingress-smoke", false, "run reversible IPv4 DNS ingress takeover smoke and exit")
	policyIngressSmokeInterface := flag.String("policy-ingress-smoke-interface", "br0", "LAN interface for ingress takeover smoke")
	policyIngressSmokeListen := flag.String("policy-ingress-smoke-listen", "192.168.10.1:55355", "explicit LAN IPv4:high-port RouterForge listener")
	policyIngressSmokeUpstream := flag.String("policy-ingress-smoke-upstream", "1.1.1.1:53", "explicit upstream for ingress takeover smoke")
	policyIngressSmokeTimeout := flag.Duration("policy-ingress-smoke-timeout", 4*time.Second, "per-query timeout for ingress takeover smoke")
	policyActivationAcceptance := flag.Bool("policy-activation-acceptance", false, "run final transaction-driver activation plus rollback acceptance and exit")
	policyActivationAcceptanceInterface := flag.String("policy-activation-acceptance-interface", "br0", "LAN interface for final activation acceptance")
	policyActivationAcceptanceListen := flag.String("policy-activation-acceptance-listen", "192.168.10.1:55356", "explicit LAN IPv4:high-port listener for final activation acceptance")
	policyActivationAcceptanceUpstream := flag.String("policy-activation-acceptance-upstream", "1.1.1.1:53", "explicit upstream for final activation acceptance")
	policyActivationAcceptanceTimeout := flag.Duration("policy-activation-acceptance-timeout", 4*time.Second, "timeout for final activation acceptance")
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
			fmt.Printf("CASE: %s TRANSPORT=%s POLICY=%s WANT_MARK=%#x REPLY=%t RCODE=%s", item.Name, item.Transport, item.Policy, item.WantMark, item.Reply, item.RCode)
			if item.Error != "" {
				fmt.Printf(" ERROR=%q", item.Error)
			}
			fmt.Println()
		}
		fmt.Printf("POLICY1_MARK_COUNT: %d\n", countDNSPolicyShadowMark(result.Marks, 0x0ffffaab))
		fmt.Printf("POLICY0_MARK_COUNT: %d\n", countDNSPolicyShadowMark(result.Marks, 0x0ffffaaa))
		fmt.Printf("STATS_REQUESTS: %d\n", result.Stats.Requests)
		fmt.Printf("STATS_SUCCESSES: %d\n", result.Stats.Successes)
		fmt.Printf("STATS_FAILURES: %d\n", result.Stats.Failures)
		fmt.Printf("STATS_SERVFAIL: %d\n", result.Stats.ServfailResponses)
		fmt.Printf("STATS_PEAK_INFLIGHT: %d\n", result.Stats.PeakInFlight)
		fmt.Printf("STATS_POLICY_SYSTEM: %d\n", result.Stats.PolicySelections["System"])
		fmt.Printf("STATS_POLICY_POLICY1: %d\n", result.Stats.PolicySelections["Policy1"])
		fmt.Printf("STATS_POLICY_POLICY0: %d\n", result.Stats.PolicySelections["Policy0"])
		if err != nil {
			fmt.Println("RESULT: FAIL")
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(4)
		}
		fmt.Println("RESULT: PASS")
		return
	}

	if *policyPersistedShadow {
		evidence, err := runDNSPolicyPersistedShadow(
			*policyPersistedShadowStore,
			*policyPersistedShadowListen,
			*policyPersistedShadowUpstream,
			*policyPersistedShadowDuration,
			*policyPersistedShadowTimeout,
			*policyPersistedShadowConcurrency,
		)
		fmt.Println("=== PERSISTED POLICY SHADOW ===")
		fmt.Printf("STORE: %s\n", evidence.StorePath)
		fmt.Printf("DOCUMENT_SHA256: %s\n", evidence.DocumentSHA256)
		fmt.Printf("DOCUMENT_VERSION: %d\n", evidence.DocumentVersion)
		fmt.Printf("DOCUMENT_UPDATED_AT: %s\n", evidence.DocumentUpdatedAt.UTC().Format(time.RFC3339Nano))
		fmt.Printf("RULE_COUNT: %d\n", evidence.RuleCount)
		fmt.Printf("INVENTORY_COUNT: %d\n", evidence.InventoryCount)
		fmt.Printf("LISTEN: %s\n", evidence.ListenAddr)
		fmt.Printf("UPSTREAM: %s\n", evidence.Upstream)
		fmt.Printf("DURATION: %s\n", evidence.Duration)
		fmt.Printf("STATS_REQUESTS: %d\n", evidence.Stats.Requests)
		fmt.Printf("STATS_SUCCESSES: %d\n", evidence.Stats.Successes)
		fmt.Printf("STATS_FAILURES: %d\n", evidence.Stats.Failures)
		fmt.Printf("STATS_SERVFAIL: %d\n", evidence.Stats.ServfailResponses)
		fmt.Printf("STATS_PEAK_INFLIGHT: %d\n", evidence.Stats.PeakInFlight)
		if err != nil {
			fmt.Println("RESULT: FAIL")
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(5)
		}
		fmt.Println("RESULT: PASS")
		return
	}

	if *policyIngressSmoke {
		result, err := runDNSPolicyIngressSmoke(*policyIngressSmokeInterface, *policyIngressSmokeListen, *policyIngressSmokeUpstream, *policyIngressSmokeTimeout)
		fmt.Println("=== POLICY INGRESS TAKEOVER SMOKE ===")
		fmt.Printf("INTERFACE: %s\n", result.Interface)
		fmt.Printf("LISTEN: %s\n", result.ListenAddr)
		fmt.Printf("INGRESS_INSTALLED: %t\n", result.IngressInstalled)
		fmt.Printf("INGRESS_VERIFIED: %t\n", result.IngressVerified)
		fmt.Printf("INGRESS_REMOVED: %t\n", result.IngressRemoved)
		fmt.Printf("POLICY1_MARK_COUNT: %d\n", result.Policy1Marks)
		fmt.Printf("POLICY0_MARK_COUNT: %d\n", result.Policy0Marks)
		fmt.Printf("STATS_REQUESTS: %d\n", result.Stats.Requests)
		fmt.Printf("STATS_SUCCESSES: %d\n", result.Stats.Successes)
		fmt.Printf("STATS_FAILURES: %d\n", result.Stats.Failures)
		fmt.Printf("STATS_SERVFAIL: %d\n", result.Stats.ServfailResponses)
		fmt.Printf("NATIVE_UDP_AFTER_ROLLBACK: %t\n", result.NativeUDPAfter)
		fmt.Printf("NATIVE_TCP_AFTER_ROLLBACK: %t\n", result.NativeTCPAfter)
		fmt.Println("--- IPTABLES RULE EVIDENCE ---")
		fmt.Print(result.RuleDump)
		if err != nil {
			fmt.Println("RESULT: FAIL")
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(6)
		}
		fmt.Println("RESULT: PASS")
		return
	}

	if *policyActivationAcceptance {
		result, err := runDNSPolicyActivationAcceptance(
			*policyActivationAcceptanceInterface,
			*policyActivationAcceptanceListen,
			*policyActivationAcceptanceUpstream,
			*policyActivationAcceptanceTimeout,
		)
		fmt.Println("=== POLICY ACTIVATION ACCEPTANCE ===")
		fmt.Printf("TX_STATE: %s\n", result.Activation.Manifest.State)
		fmt.Printf("ACTIVATED: %t\n", result.Activation.Activated)
		fmt.Printf("SNAPSHOT_IDENTITY: %s\n", result.Activation.Snapshot.Identity)
		fmt.Printf("ROLLBACK_AFTER_COMMIT: %t\n", result.RolledBack)
		fmt.Printf("NATIVE_AFTER_ROLLBACK: %t\n", result.NativeOK)
		if err != nil {
			fmt.Println("RESULT: FAIL")
			fmt.Printf("ERROR: %v\n", err)
			os.Exit(7)
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
		fast := time.NewTicker(2 * time.Second)
		dedup := time.NewTicker(10 * time.Second)
		flush := time.NewTicker(30 * time.Second)
		defer fast.Stop()
		defer dedup.Stop()
		defer flush.Stop()

		for {
			select {
			case now := <-fast.C:
				store.CleanupTransient(now, eventLog)
				plainDNS.Sweep(now)
			case now := <-dedup.C:
				store.CleanupLongLived(now)
			case <-flush.C:
				eventLog.FlushDNSFailures()
			}
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
