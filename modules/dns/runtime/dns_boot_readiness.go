//go:build routerforge_dns_module

package main

import (
	"context"
	"fmt"
	"time"
)

const dnsBootReadinessWindow = 60 * time.Second
const dnsBootReadinessInterval = 3 * time.Second

type dnsBootProbe func() error

func ndmcBootProbe() error {
	_, err := ndmcOutput("show dns-proxy", 3*time.Second)
	return err
}

// waitDNSBootReadiness only gates initial background collection. The HTTP
// server and packet capture continue starting independently. Normal adaptive
// polling still retries after timeout, so a late ndmc recovery is supported.
func waitDNSBootReadiness(probe dnsBootProbe, window, interval time.Duration, sleep func(time.Duration)) (int, error) {
	if probe == nil || sleep == nil || window < 0 || interval <= 0 {
		return 0, fmt.Errorf("invalid DNS boot readiness configuration")
	}
	attempts := 0
	remaining := window
	for {
		attempts++
		if err := probe(); err == nil {
			return attempts, nil
		} else if remaining <= 0 {
			return attempts, fmt.Errorf("ndmc unavailable after %d attempts: %w", attempts, err)
		}
		delay := interval
		if delay > remaining {
			delay = remaining
		}
		sleep(delay)
		remaining -= delay
	}
}

func dnsBootGate(log *EventLogger, probe dnsBootProbe) {
	if err := dnsWaitColdBoot(context.Background()); err != nil {
		log.Event("DNS_BOOT_DELAY_CANCELLED", err.Error())
		return
	}
	attempts, err := waitDNSBootReadiness(probe, dnsBootReadinessWindow, dnsBootReadinessInterval, time.Sleep)
	dnsEndBootProbe()
	if err != nil {
		log.Event("DNS_BOOT_READINESS_TIMEOUT", err.Error())
	} else {
		log.Event("DNS_BOOT_READINESS_READY", fmt.Sprintf("attempts=%d", attempts))
	}
}

func startDNSCollectorsAfterReadiness(store *Store, interval time.Duration, log *EventLogger, probe dnsBootProbe) {
	dnsBootGate(log, probe)
	go discoveryLoop(store, interval, log)
	clientRegistryLoop(store, log)
}
