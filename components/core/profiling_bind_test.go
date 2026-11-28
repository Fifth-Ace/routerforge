package main

import (
	"net"
	"net/http"
	"testing"
	"time"
)

func TestProfilingListenerBindFailure(t *testing.T) {
	blocker, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer blocker.Close()
	state := &profilingRuntime{Enabled: true, Listen: blocker.Addr().String(), SlowMS: 750}
	server := &http.Server{Addr: blocker.Addr().String()}
	if e = startProfilingListener(state, server); e == nil {
		t.Fatal("occupied address accepted")
	}
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.Running || state.Error == "" || !state.StartedAt.IsZero() {
		t.Fatal("unbound listener reported running")
	}
}
func TestProfilingListenerRunningOnlyAfterBind(t *testing.T) {
	state := &profilingRuntime{Enabled: true, Listen: "127.0.0.1:0", SlowMS: 750}
	server := &http.Server{Addr: state.Listen, Handler: http.NewServeMux()}
	if e := startProfilingListener(state, server); e != nil {
		t.Fatal(e)
	}
	state.mu.RLock()
	running := state.Running
	started := state.StartedAt
	state.mu.RUnlock()
	if !running || started.IsZero() {
		t.Fatal("bound listener not reported")
	}
	if e := server.Close(); e != nil {
		t.Fatal(e)
	}
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		state.mu.RLock()
		running = state.Running
		state.mu.RUnlock()
		if !running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("closed listener still running")
}
