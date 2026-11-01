package lite

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// FirewallIO is deliberately small: tests inject a stateful kernel model and
// production always uses the shared non-shell safety executor.
type FirewallIO func(context.Context, string, ...string) (string, error)

type XTables struct {
	Exec  FirewallIO
	mu    sync.Mutex
	modes map[string]int // 0: -w seconds, 1: legacy -w, 2: no wait support
	Delay func(context.Context, time.Duration) error
}

type LockBusyError struct{ Cause error }

func (e *LockBusyError) Error() string { return "xtables lock budget exhausted: " + e.Cause.Error() }
func (e *LockBusyError) Unwrap() error { return e.Cause }

func lockContention(err error, output string) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(output + " " + err.Error())
	return strings.Contains(s, "xtables lock") || strings.Contains(s, "xtables.lock") || strings.Contains(s, "another app is currently holding") || strings.Contains(s, "resource temporarily unavailable")
}
func waitUnsupported(err error, output string) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(output + " " + err.Error())
	return (strings.Contains(s, "unknown option") || strings.Contains(s, "unrecognized option") || strings.Contains(s, "invalid option") || strings.Contains(s, "bad argument")) && (strings.Contains(s, "wait") || strings.Contains(s, "-w") || strings.Contains(s, "`1'") || strings.Contains(s, "'1'"))
}
func waitContext(ctx context.Context, delay time.Duration) error {
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func systemFirewallIO(ctx context.Context, name string, args ...string) (string, error) {
	child, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	b, err := safety.RunCommand(child, 1<<20, name, args...)
	if len(b) >= 1<<20 {
		return "", errors.New("firewall inspection exceeds output limit")
	}
	if err != nil {
		return string(b), fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(b)))
	}
	return string(b), nil
}
func (x *XTables) Run(ctx context.Context, name string, args ...string) (string, error) {
	if x.Exec == nil {
		return "", errors.New("firewall executor unavailable")
	}
	iptables := name == "iptables" || name == "ip6tables"
	x.mu.Lock()
	mode := x.modes[name]
	x.mu.Unlock()
	var output string
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return output, ctx.Err()
		}
		argv := append([]string{}, args...)
		if iptables && mode < 2 {
			prefix := []string{"-w"}
			if mode == 0 {
				prefix = append(prefix, "1")
			}
			argv = append(prefix, argv...)
		}
		output, err = x.Exec(ctx, name, argv...)
		if err == nil {
			x.mu.Lock()
			if x.modes == nil {
				x.modes = map[string]int{}
			}
			x.modes[name] = mode
			x.mu.Unlock()
			return output, nil
		}
		if iptables && mode < 2 && waitUnsupported(err, output) {
			// The option was rejected before command execution. The next
			// variant is still bounded by the caller and per-process timeout.
			mode++
			attempt--
			continue
		}
		if !iptables || !lockContention(err, output) {
			return output, err
		}
		if attempt < 2 {
			delay := x.Delay
			if delay == nil {
				delay = waitContext
			}
			if e := delay(ctx, time.Duration(attempt+1)*75*time.Millisecond); e != nil {
				return output, e
			}
		}
	}
	return output, &LockBusyError{Cause: err}
}
