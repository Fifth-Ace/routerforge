package lite

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type fakeChild struct {
	done    chan struct{}
	stopped atomic.Bool
}

func (c *fakeChild) Wait() error { <-c.done; return errors.New("simulated child exit") }
func (c *fakeChild) Terminate() error {
	if c.stopped.CompareAndSwap(false, true) {
		close(c.done)
	}
	return nil
}
func TestP19SupervisorRestartsCrashAndBoundsRecovery(t *testing.T) {
	starts, cleanups := 0, 0
	s := Supervisor{MaxRestarts: 2, Delay: func(context.Context, time.Duration) error { return nil }, Cleanup: func(context.Context) error { cleanups++; return nil }, Start: func() (SupervisedChild, error) {
		starts++
		c := &fakeChild{done: make(chan struct{})}
		c.Terminate()
		return c, nil
	}}
	if err := s.Run(context.Background()); err == nil || starts != 3 || cleanups != 4 {
		t.Fatalf("unbounded or missing crash cleanup: starts=%d cleanup=%d err=%v", starts, cleanups, err)
	}
}
func TestP19SupervisorMaintenanceSignalDoesNotRestart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	starts := atomic.Int32{}
	cleanups := atomic.Int32{}
	ready := make(chan *fakeChild, 1)
	s := Supervisor{Cleanup: func(context.Context) error { cleanups.Add(1); return nil }, Start: func() (SupervisedChild, error) {
		starts.Add(1)
		c := &fakeChild{done: make(chan struct{})}
		ready <- c
		return c, nil
	}}
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	child := <-ready
	cancel()
	select {
	case e := <-done:
		if e != nil || starts.Load() != 1 || !child.stopped.Load() || cleanups.Load() != 2 {
			t.Fatal("maintenance restarted a child", e)
		}
	case <-time.After(time.Second):
		t.Fatal("supervisor ignored maintenance stop")
	}
}
