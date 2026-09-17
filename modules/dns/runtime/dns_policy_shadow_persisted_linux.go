//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"time"
)

func runDNSPolicyPersistedShadow(
	storePath, listenAddr, upstream string,
	duration, timeout time.Duration,
	maxConcurrent int,
) (DNSPolicyPersistedShadowEvidence, error) {
	if duration <= 0 || duration > 10*time.Minute {
		return DNSPolicyPersistedShadowEvidence{}, fmt.Errorf("persisted shadow duration must be >0 and <=10m")
	}
	raw, err := os.ReadFile(storePath)
	if err != nil {
		return DNSPolicyPersistedShadowEvidence{}, fmt.Errorf("read persisted shadow document: %w", err)
	}
	sum := sha256.Sum256(raw)

	store := newDNSPolicyStore(storePath)
	doc, err := store.Load()
	if err != nil {
		return DNSPolicyPersistedShadowEvidence{}, err
	}
	inventory, err := readDNSPolicyInventory()
	if err != nil {
		return DNSPolicyPersistedShadowEvidence{}, fmt.Errorf("read live policy inventory: %w", err)
	}
	cfg, err := prepareDNSPolicyPersistedShadowConfig(doc, inventory, listenAddr, upstream, timeout, maxConcurrent)
	if err != nil {
		return DNSPolicyPersistedShadowEvidence{}, err
	}

	server, err := newDNSPolicyShadowServer(cfg, discoverPolicyRoutes)
	if err != nil {
		return DNSPolicyPersistedShadowEvidence{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	if err := server.Serve(ctx); err != nil {
		return DNSPolicyPersistedShadowEvidence{}, err
	}

	return DNSPolicyPersistedShadowEvidence{
		StorePath:         storePath,
		DocumentSHA256:    fmt.Sprintf("%x", sum),
		DocumentVersion:   doc.Version,
		DocumentUpdatedAt: doc.UpdatedAt,
		RuleCount:         len(cfg.Rules),
		InventoryCount:    len(inventory),
		ListenAddr:        cfg.ListenAddr,
		Upstream:          cfg.Upstream,
		Duration:          duration,
		Stats:             server.Stats(),
	}, nil
}
