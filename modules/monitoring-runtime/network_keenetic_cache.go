package main

import (
	"sync"
	"time"
)

type keeneticSnapshotCache[T any] struct {
	mu         sync.Mutex
	ttl        time.Duration
	snapshot   []T
	scanned    time.Time
	refreshing bool
	ready      chan struct{}
	read       func() ([]T, error)
}

func newKeeneticSnapshotCache[T any](ttl time.Duration, read func() ([]T, error)) *keeneticSnapshotCache[T] {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	if read == nil {
		panic("keenetic snapshot reader is required")
	}
	return &keeneticSnapshotCache[T]{ttl: ttl, read: read}
}

// snapshotForRequest bounds expensive Keenetic metadata reads on HTTP paths.
// The first request fills the cache synchronously. Once attempted, stale reads
// return the current snapshot immediately while exactly one refresh runs.
func (c *keeneticSnapshotCache[T]) snapshotForRequest() []T {
	c.mu.Lock()
	if !c.scanned.IsZero() {
		snapshot := append([]T(nil), c.snapshot...)
		stale := time.Since(c.scanned) >= c.ttl
		if stale && !c.refreshing {
			c.refreshing = true
			c.ready = make(chan struct{})
			go c.refresh()
		}
		c.mu.Unlock()
		return snapshot
	}

	if c.refreshing {
		ready := c.ready
		c.mu.Unlock()
		<-ready

		c.mu.Lock()
		snapshot := append([]T(nil), c.snapshot...)
		c.mu.Unlock()
		return snapshot
	}

	c.refreshing = true
	c.ready = make(chan struct{})
	c.mu.Unlock()

	c.refresh()

	c.mu.Lock()
	snapshot := append([]T(nil), c.snapshot...)
	c.mu.Unlock()
	return snapshot
}

func (c *keeneticSnapshotCache[T]) refresh() {
	items, err := c.read()
	now := time.Now()

	c.mu.Lock()
	if err == nil {
		c.snapshot = append([]T(nil), items...)
	}
	// Successful empty snapshots and failed reads both get a bounded retry
	// window. On failures, keep serving the last good snapshot instead of
	// restarting an expensive reader on every frontend poll.
	c.scanned = now
	c.refreshing = false
	if c.ready != nil {
		close(c.ready)
		c.ready = nil
	}
	c.mu.Unlock()
}
