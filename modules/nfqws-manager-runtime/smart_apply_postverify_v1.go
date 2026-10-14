package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// NF4C verifies the actual target via normal production routing *inside* the
// Smart Apply transaction. On failure the caller invokes existing rollback.
// A fixed, already live-resolved IPv4 avoids a second DNS route or target swap.
func nf4cValidatePostApplyTarget(target, ip string) error {
	if target == "" && ip == "" {
		return nil // Non-AutoTune Smart Apply callers keep their existing contract.
	}
	if target == "" || ip == "" {
		return errors.New("post-apply verification requires both target and IPv4")
	}
	normalized, err := v2NormalizeTarget(target)
	if err != nil || normalized != target {
		return errors.New("post-apply target must be a normalized DNS hostname")
	}
	address := net.ParseIP(ip)
	if address == nil || address.To4() == nil || !address.IsGlobalUnicast() ||
		address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() ||
		address.String() != ip {
		return errors.New("post-apply IPv4 must be a canonical public address")
	}
	return nil
}

// Reuse the direct selector's existing HTTPS/HTTP progress criteria, without
// spinning up a second NFQUEUE sandbox or trusting cached historical results.
// One of two short production-routed probes must prove a real TLS+HTTP reply.
func nf4cVerifyHTTPSWithProbe(
	ctx context.Context, target, ip string,
	probe func(context.Context, string, string, int) (v2HTTPMetrics, error),
) error {
	if err := nf4cValidatePostApplyTarget(target, ip); err != nil {
		return err
	}
	if target == "" {
		return errors.New("post-apply verification target is absent")
	}
	if probe == nil {
		return errors.New("post-apply probe is unavailable")
	}
	last := "no HTTPS response"
	for attempt := 0; attempt < 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("post-apply verification interrupted: %w", err)
		}
		attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		metrics, err := probe(attemptCtx, target, ip, 0)
		cancel()
		if err == nil && metrics.TLSComplete && metrics.ProgressProven && metrics.HTTPStatus > 0 {
			return nil
		}
		if err != nil {
			last = err.Error()
		} else {
			last = "TLS and complete/progressing HTTP reply not proven"
		}
		if attempt == 0 {
			timer := time.NewTimer(time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return fmt.Errorf("post-apply verification interrupted: %w", ctx.Err())
			}
		}
	}
	return errors.New("target did not open through production routing: " + last)
}

func nf4cVerifyPostApply(ctx context.Context, target, ip string) error {
	return nf4cVerifyHTTPSWithProbe(ctx, target, ip, v2ReadHTTPS)
}
