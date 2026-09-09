package main

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type cacheTestItem struct {
	Name string
}

func TestKeeneticSnapshotCacheColdLoadSingleflight(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	cache := newKeeneticSnapshotCache(time.Minute, func() ([]cacheTestItem, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return []cacheTestItem{{Name: "cold"}}, nil
	})

	const readers = 12
	results := make(chan []cacheTestItem, readers)
	for i := 0; i < readers; i++ {
		go func() { results <- cache.snapshotForRequest() }()
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("cold cache read did not start")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cold reads=%d want=1", got)
	}

	close(release)
	for i := 0; i < readers; i++ {
		got := <-results
		if len(got) != 1 || got[0].Name != "cold" {
			t.Fatalf("reader %d got %#v", i, got)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("cold reads after release=%d want=1", got)
	}
}

func TestKeeneticSnapshotCacheStaleWhileSingleRefreshRuns(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	cache := newKeeneticSnapshotCache(time.Minute, func() ([]cacheTestItem, error) {
		n := calls.Add(1)
		if n == 1 {
			return []cacheTestItem{{Name: "old"}}, nil
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return []cacheTestItem{{Name: "new"}}, nil
	})

	prime := cache.snapshotForRequest()
	if len(prime) != 1 || prime[0].Name != "old" {
		t.Fatalf("prime=%#v", prime)
	}

	cache.mu.Lock()
	cache.scanned = time.Now().Add(-2 * cache.ttl)
	cache.mu.Unlock()

	returned := make(chan []cacheTestItem, 1)
	go func() { returned <- cache.snapshotForRequest() }()

	select {
	case stale := <-returned:
		if len(stale) != 1 || stale[0].Name != "old" {
			t.Fatalf("stale=%#v", stale)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("stale request blocked on refresh")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background refresh did not start")
	}

	for i := 0; i < 16; i++ {
		got := cache.snapshotForRequest()
		if len(got) != 1 || got[0].Name != "old" {
			t.Fatalf("stale reader %d=%#v", i, got)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("reads while refresh blocked=%d want=2", got)
	}

	close(release)
	deadline := time.Now().Add(time.Second)
	for {
		got := cache.snapshotForRequest()
		if len(got) == 1 && got[0].Name == "new" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("new snapshot not published: %#v", got)
		}
		time.Sleep(time.Millisecond)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("reads after publish=%d want=2", got)
	}
}

func TestKeeneticSnapshotCacheCachesSuccessfulEmptySnapshot(t *testing.T) {
	var calls atomic.Int32
	cache := newKeeneticSnapshotCache(time.Minute, func() ([]cacheTestItem, error) {
		calls.Add(1)
		return nil, nil
	})

	if got := cache.snapshotForRequest(); len(got) != 0 {
		t.Fatalf("first empty snapshot=%#v", got)
	}
	for i := 0; i < 10; i++ {
		if got := cache.snapshotForRequest(); len(got) != 0 {
			t.Fatalf("cached empty snapshot %d=%#v", i, got)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("empty snapshot reads=%d want=1", got)
	}
}

func TestKeeneticSnapshotCacheFailedRefreshKeepsLastGoodAndBacksOff(t *testing.T) {
	var calls atomic.Int32
	failed := make(chan struct{})

	cache := newKeeneticSnapshotCache(time.Minute, func() ([]cacheTestItem, error) {
		n := calls.Add(1)
		if n == 1 {
			return []cacheTestItem{{Name: "good"}}, nil
		}
		close(failed)
		return nil, errors.New("boom")
	})

	prime := cache.snapshotForRequest()
	if len(prime) != 1 || prime[0].Name != "good" {
		t.Fatalf("prime=%#v", prime)
	}

	cache.mu.Lock()
	cache.scanned = time.Now().Add(-2 * cache.ttl)
	cache.mu.Unlock()

	stale := cache.snapshotForRequest()
	if len(stale) != 1 || stale[0].Name != "good" {
		t.Fatalf("stale=%#v", stale)
	}

	select {
	case <-failed:
	case <-time.After(time.Second):
		t.Fatal("failed refresh did not execute")
	}

	deadline := time.Now().Add(time.Second)
	for {
		cache.mu.Lock()
		refreshing := cache.refreshing
		cache.mu.Unlock()
		if !refreshing {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed refresh did not finish")
		}
		time.Sleep(time.Millisecond)
	}

	for i := 0; i < 10; i++ {
		got := cache.snapshotForRequest()
		if len(got) != 1 || got[0].Name != "good" {
			t.Fatalf("post-failure snapshot %d=%#v", i, got)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("reads after failed refresh=%d want=2", got)
	}
}
