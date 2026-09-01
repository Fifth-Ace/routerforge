package main

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

const healthPrefix = "dnsmon-"

func isHealthDomain(s string) bool { return strings.HasPrefix(strings.ToLower(s), healthPrefix) }

func healthLoop(store *Store, interval time.Duration, log *EventLogger) {
	// Discovery runs immediately. Give it enough time to populate the first map.
	time.Sleep(3 * time.Second)

	checkAll := func() {
		snap := store.Snapshot(0, 0, 0)
		ups, _ := snap["upstreams"].([]UpstreamView)

		// Avoid a burst of dozens of simultaneous encrypted-DNS probes on a small router.
		sem := make(chan struct{}, 6)
		var wg sync.WaitGroup
		for _, u := range ups {
			u := u
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				checkOne(store, u, log)
			}()
		}
		wg.Wait()
	}

	checkAll()
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		checkAll()
	}
}

func healthDomain(u UpstreamView) string {
	base := strings.TrimSpace(u.Domain)
	base = strings.TrimPrefix(base, "*.")
	base = strings.TrimSuffix(base, ".")
	if base == "" {
		// A real, stable domain is friendlier to resolvers than RFC-reserved .invalid,
		// while the random dnsmon-* label keeps the probe out of normal flow stats.
		base = "example.com"
	}
	return fmt.Sprintf("%s%x.%s", healthPrefix, rand.Uint64(), base)
}

func checkOne(store *Store, u UpstreamView, log *EventLogger) {
	id := uint16(rand.Intn(65535))
	name := healthDomain(u)
	q := buildDNSQuery(id, name, 1)
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(u.Port)}
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		store.SetHealth(u.Port, time.Now(), 0, err, log)
		return
	}
	defer c.Close()

	// Respect Keenetic's own resolver timeout. Clamp only pathological values.
	timeout := 5 * time.Second
	if u.TimeoutMS > 0 {
		timeout = time.Duration(u.TimeoutMS) * time.Millisecond
	}
	if timeout < 1*time.Second {
		timeout = 1 * time.Second
	}
	if timeout > 8*time.Second {
		timeout = 8 * time.Second
	}

	_ = c.SetDeadline(time.Now().Add(timeout))
	start := time.Now()
	if _, err = c.Write(q); err != nil {
		store.SetHealth(u.Port, time.Now(), 0, err, log)
		return
	}
	buf := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		store.SetHealth(u.Port, time.Now(), 0, err, log)
		return
	}
	lat := float64(time.Since(start).Microseconds()) / 1000
	if n < 12 || binary.BigEndian.Uint16(buf[:2]) != id {
		store.SetHealth(u.Port, time.Now(), lat, fmt.Errorf("invalid DNS response"), log)
		return
	}
	d, ok := parseDNSMessage(buf[:n])
	if !ok || !d.QR {
		store.SetHealth(u.Port, time.Now(), lat, fmt.Errorf("invalid DNS response"), log)
		return
	}
	if d.RCode == 2 || d.RCode == 5 {
		store.SetHealth(u.Port, time.Now(), lat, fmt.Errorf("DNS %s", rcodeName(d.RCode)), log)
		return
	}
	store.SetHealth(u.Port, time.Now(), lat, nil, log)
}
