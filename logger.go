package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type dnsErrorBurst struct {
	kind        string
	profile     string
	upstream    string
	windowStart time.Time
	count       int
	domains     map[string]struct{}
}

type EventLogger struct {
	mu      sync.Mutex
	path    string
	maxSize int64
	bursts  map[string]*dnsErrorBurst
}

func NewEventLogger(path string) *EventLogger {
	return &EventLogger{path: path, maxSize: 1024 * 1024, bursts: make(map[string]*dnsErrorBurst)}
}

func (l *EventLogger) writeLocked(kind, msg string) {
	_ = os.MkdirAll(filepath.Dir(l.path), 0755)
	if st, err := os.Stat(l.path); err == nil && st.Size() >= l.maxSize {
		_ = os.Remove(l.path + ".1")
		_ = os.Rename(l.path, l.path+".1")
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %-16s %s\n", time.Now().Format("2006-01-02 15:04:05"), kind, msg)
}

func (l *EventLogger) Event(kind, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writeLocked(kind, msg)
}

// DNSError aggregates noisy per-domain DNS failures on disk. Individual failures
// are still counted by Store and remain available to the web UI in RAM.
func (l *EventLogger) DNSError(kind, profile, upstream, domain string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	key := kind + "\x00" + profile + "\x00" + upstream
	b := l.bursts[key]
	if b != nil && now.Sub(b.windowStart) >= time.Minute {
		l.flushBurstLocked(key, b)
		b = nil
	}
	if b == nil {
		b = &dnsErrorBurst{kind: kind, profile: profile, upstream: upstream, windowStart: now, domains: make(map[string]struct{})}
		l.bursts[key] = b
	}
	b.count++
	if domain != "" && len(b.domains) < 1000 {
		b.domains[domain] = struct{}{}
	}
}

func (l *EventLogger) FlushDNSFailures() {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	for key, b := range l.bursts {
		if now.Sub(b.windowStart) >= time.Minute {
			l.flushBurstLocked(key, b)
		}
	}
}

func (l *EventLogger) flushBurstLocked(key string, b *dnsErrorBurst) {
	if b == nil || b.count == 0 {
		delete(l.bursts, key)
		return
	}
	domains := make([]string, 0, len(b.domains))
	for d := range b.domains {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	sample := ""
	if len(domains) > 0 {
		n := len(domains)
		if n > 3 {
			n = 3
		}
		sample = " / sample=" + strings.Join(domains[:n], ",")
	}
	msg := fmt.Sprintf("%s / %s / %s x%d / %d domains%s", b.profile, b.upstream, b.kind, b.count, len(domains), sample)
	l.writeLocked(b.kind+"_BURST", msg)
	delete(l.bursts, key)
}
