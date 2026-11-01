package lite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

type SupervisedChild interface {
	Wait() error
	Terminate() error
}
type Supervisor struct {
	Start       func() (SupervisedChild, error)
	Cleanup     func(context.Context) error
	Delay       func(context.Context, time.Duration) error
	Log         io.Writer
	MaxRestarts int
}

func (s Supervisor) Run(ctx context.Context) error {
	limit := s.MaxRestarts
	if limit <= 0 {
		limit = 5
	}
	wait := s.Delay
	if wait == nil {
		wait = waitContext
	}
	restarts := []time.Time{}
	afterExit := func() error {
		if s.Cleanup == nil {
			return nil
		}
		cleanup, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return s.Cleanup(cleanup)
	}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if s.Cleanup != nil {
			cleanup, cancel := context.WithTimeout(ctx, 8*time.Second)
			err := s.Cleanup(cleanup)
			cancel()
			if err != nil {
				return fmt.Errorf("supervisor cleanup blocked: %w", err)
			}
		}
		child, err := s.Start()
		if err != nil {
			return err
		}
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()
		select {
		case <-ctx.Done():
			if e := child.Terminate(); e != nil {
				return e
			}
			timer := time.NewTimer(10 * time.Second)
			select {
			case <-done:
				timer.Stop()
				return afterExit()
			case <-timer.C:
				if s.Log != nil {
					fmt.Fprintln(s.Log, "DNS_SUPERVISOR: child stop timed out; retaining process ownership, no SIGKILL")
				}
				<-done
				return afterExit()
			}
		case err = <-done:
			if ctx.Err() != nil {
				return afterExit()
			}
			if s.Log != nil {
				fmt.Fprintln(s.Log, "DNS_SUPERVISOR: unexpected child exit:", err)
			}
		}
		now := time.Now()
		recent := restarts[:0]
		for _, at := range restarts {
			if now.Sub(at) < time.Minute {
				recent = append(recent, at)
			}
		}
		restarts = recent
		if len(restarts) >= limit {
			if s.Cleanup != nil {
				cleanup, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				_ = s.Cleanup(cleanup)
				cancel()
			}
			return errors.New("DNS supervisor restart budget exhausted")
		}
		restarts = append(restarts, now)
		if err = wait(ctx, time.Duration(len(restarts))*250*time.Millisecond); err != nil {
			return nil
		}
	}
}
