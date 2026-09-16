package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
)

var adminConfigVaultMutationMu sync.Mutex

type adminConfigVaultPrechangeSnapshot struct {
	Protected     bool
	SnapshotID    string
	TransactionID string
	Artifacts     int
	EmptyBaseline bool
}

func adminConfigVaultPathWithinRoot(root, path string) bool {
	root = filepath.Clean(strings.TrimSpace(root))
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return false
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return relative == "." ||
		(relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func adminConfigVaultManagedMutation(paths ...string) bool {
	for _, path := range paths {
		if adminConfigVaultPathWithinRoot(adminConfigVaultManagedRoot, path) {
			return true
		}
	}
	return false
}

func lockAdminConfigVaultMutation(paths ...string) (func(), bool) {
	if !adminConfigVaultManagedMutation(paths...) {
		return func() {}, false
	}
	adminConfigVaultMutationMu.Lock()
	return adminConfigVaultMutationMu.Unlock, true
}

func captureAdminConfigVaultPrechangeLocked(action string, paths ...string) (adminConfigVaultPrechangeSnapshot, error) {
	if !adminConfigVaultManagedMutation(paths...) {
		return adminConfigVaultPrechangeSnapshot{}, nil
	}

	managed, specs, err := discoverAdminConfigVaultArtifacts()
	if err != nil {
		return adminConfigVaultPrechangeSnapshot{}, err
	}
	store, err := newAdminConfigVault()
	if err != nil {
		return adminConfigVaultPrechangeSnapshot{}, err
	}

	action = strings.TrimSpace(action)
	if action == "" {
		action = "managed-mutation"
	}
	now := time.Now().UTC()
	transactionID := fmt.Sprintf("auto-prechange-%d", now.UnixNano())
	manifest, err := store.Capture(configvault.CaptureRequest{
		Component:     "admin",
		Reason:        "automatic pre-change snapshot: " + action,
		TransactionID: transactionID,
		Artifacts:     specs,
	})
	if err != nil {
		return adminConfigVaultPrechangeSnapshot{}, err
	}

	result := adminConfigVaultPrechangeSnapshot{
		Protected:     true,
		SnapshotID:    manifest.ID,
		TransactionID: transactionID,
		Artifacts:     len(managed),
		EmptyBaseline: len(managed) == 0,
	}
	log.Printf(
		"routerforge-admin config-vault prechange action=%q snapshot=%q transaction=%q artifacts=%d empty_baseline=%t",
		action,
		result.SnapshotID,
		result.TransactionID,
		result.Artifacts,
		result.EmptyBaseline,
	)
	return result, nil
}
