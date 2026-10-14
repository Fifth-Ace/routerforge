package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	nf4dMaxRegressionGuards = 2
	nf4dMaxMemoryAge        = 24 * time.Hour
)

type nf4dRegressionGuard struct {
	Target string
	IPv4   string
}

// Memory is only a source of possible control sites. It never counts as proof
// of production reachability: each selected site must pass a fresh precheck.
func nf4dSelectRegressionCandidates(entries []v2TargetMemoryEntry, changedTarget string, now time.Time) []string {
	copyEntries := append([]v2TargetMemoryEntry(nil), entries...)
	sort.SliceStable(copyEntries, func(i, j int) bool { return copyEntries[i].LastVerified > copyEntries[j].LastVerified })
	seen := map[string]bool{}
	out := []string{}
	for _, entry := range copyEntries {
		if entry.ResultClass != "WORKING" || entry.Protocol != benchTransportHTTPS ||
			entry.VerifiedCount < 2 || !entry.InfrastructureOK || !entry.CleanupProven {
			continue
		}
		verifiedAt, err := time.Parse(time.RFC3339, entry.LastVerified)
		if err != nil || verifiedAt.After(now) || now.Sub(verifiedAt) > nf4dMaxMemoryAge {
			continue
		}
		name, err := v2NormalizeTarget(entry.Target)
		if err != nil || name != entry.Target || name == changedTarget || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
		if len(out) == nf4dMaxRegressionGuards {
			break
		}
	}
	return out
}

func nf4dRegressionCandidates(changedTarget string) []string {
	memory, err := readV2TargetMemory()
	if err != nil {
		return nil // Optional regression evidence never blocks ordinary Apply.
	}
	return nf4dSelectRegressionCandidates(memory.Entries, changedTarget, time.Now().UTC())
}

type nf4dResolver func(context.Context, string) (string, error)
type nf4dHTTPSProbe func(context.Context, string, string, int) (v2HTTPMetrics, error)

func nf4dProbeOK(metrics v2HTTPMetrics, err error) bool {
	return err == nil && metrics.TLSComplete && metrics.ProgressProven && metrics.HTTPStatus > 0
}

// Only fresh, successful production-routed HTTPS probes become guards. Resolve
// each site once, then use the same pinned public IPv4 before and after Apply.
func nf4dPrepareRegressionGuards(
	ctx context.Context, changedTarget string, names []string,
	resolve nf4dResolver, probe nf4dHTTPSProbe,
) ([]nf4dRegressionGuard, error) {
	if len(names) == 0 {
		return nil, nil
	}
	if changedTarget == "" || len(names) > nf4dMaxRegressionGuards || resolve == nil || probe == nil {
		return nil, errors.New("invalid bounded regression guard request")
	}
	if normalized, err := v2NormalizeTarget(changedTarget); err != nil || normalized != changedTarget {
		return nil, errors.New("regression guard target is invalid")
	}
	seen := make(map[string]bool)
	for _, name := range names {
		normalized, err := v2NormalizeTarget(name)
		if err != nil || normalized != name || normalized == changedTarget || seen[name] {
			return nil, errors.New("regression guard contains invalid or duplicate hostname")
		}
		seen[name] = true
	}
	guards := make([]nf4dRegressionGuard, 0, len(names))
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("regression precheck interrupted: %w", err)
		}
		resolveCtx, cancelResolve := context.WithTimeout(ctx, 4*time.Second)
		ip, err := resolve(resolveCtx, name)
		cancelResolve()
		if err != nil || nf4cValidatePostApplyTarget(name, ip) != nil {
			continue
		}
		probeCtx, cancelProbe := context.WithTimeout(ctx, 12*time.Second)
		metrics, probeErr := probe(probeCtx, name, ip, 0)
		cancelProbe()
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("regression precheck interrupted: %w", err)
		}
		if nf4dProbeOK(metrics, probeErr) {
			guards = append(guards, nf4dRegressionGuard{Target: name, IPv4: ip})
		}
	}
	return guards, nil
}

func nf4dVerifyRegressionGuards(ctx context.Context, guards []nf4dRegressionGuard, probe nf4dHTTPSProbe) error {
	if len(guards) > nf4dMaxRegressionGuards || probe == nil && len(guards) > 0 {
		return errors.New("invalid post-apply regression guard state")
	}
	for _, guard := range guards {
		if err := nf4cValidatePostApplyTarget(guard.Target, guard.IPv4); err != nil {
			return err
		}
		if err := nf4cVerifyHTTPSWithProbe(ctx, guard.Target, guard.IPv4, probe); err != nil {
			return fmt.Errorf("previously working site %s failed after Apply: %w", guard.Target, err)
		}
	}
	return nil
}

func nf4dResolveProductionGuard(ctx context.Context, target string) (string, error) {
	return resolveBenchServerIPv4(ctx, strings.TrimSpace(target))
}
