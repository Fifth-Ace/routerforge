package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/configvault"
	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
	"github.com/Fifth-Ace/routerforge/internal/safety"
)

type adminConfigVaultRestoreRequest struct {
	SnapshotID        string `json:"snapshot_id"`
	ConfirmSnapshotID string `json:"confirm_snapshot_id"`
	TransactionID     string `json:"transaction_id"`
	Confirm           string `json:"confirm"`
}

type adminConfigVaultRestoreOutcome struct {
	OK               bool                 `json:"ok"`
	TargetSnapshot   string               `json:"target_snapshot"`
	SafetySnapshot   string               `json:"safety_snapshot,omitempty"`
	Transaction      transaction.Manifest `json:"transaction"`
	RolledBack       bool                 `json:"rolled_back"`
	RollbackVerified bool                 `json:"rollback_verified"`
	ReadbackVerified bool                 `json:"readback_verified"`
	Probe            string               `json:"probe"`
	Error            string               `json:"error,omitempty"`
}

func handleAdminConfigVaultRestore(w http.ResponseWriter, r *http.Request) {
	var request adminConfigVaultRestoreRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid config vault restore request"})
		return
	}
	request.SnapshotID = strings.TrimSpace(request.SnapshotID)
	request.ConfirmSnapshotID = strings.TrimSpace(request.ConfirmSnapshotID)
	request.TransactionID = strings.TrimSpace(request.TransactionID)

	if request.Confirm != "RESTORE" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal RESTORE"})
		return
	}
	if request.SnapshotID == "" || request.ConfirmSnapshotID != request.SnapshotID {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm_snapshot_id must exactly match snapshot_id"})
		return
	}
	if len(request.TransactionID) > adminConfigVaultTransactionMaxLen {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "transaction_id is too long"})
		return
	}

	store, err := newAdminConfigVault()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	target, err := store.Get(request.SnapshotID)
	if errors.Is(err, os.ErrNotExist) {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "snapshot not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	if target.Component != "admin" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "snapshot component is not restorable by Admin"})
		return
	}
	if err := validateAdminConfigVaultRestoreManifest(adminConfigVaultManagedRoot, target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "restore precheck failed: " + err.Error()})
		return
	}

	now := time.Now().UTC()
	transactionID := request.TransactionID
	if transactionID == "" {
		transactionID = fmt.Sprintf("restore-%d", now.UnixNano())
	}
	tx := transaction.Manifest{
		ID:        transactionID,
		Component: "admin",
		State:     transaction.Precheck,
		StartedAt: now,
		UpdatedAt: now,
		Artifacts: []string{target.ID},
	}

	_, currentSpecs, err := discoverAdminConfigVaultArtifacts()
	if err != nil {
		_ = transaction.Advance(&tx, transaction.Failed)
		writeAdminConfigVaultRestoreFailure(w, target.ID, "", tx, false, false, "precheck", err)
		return
	}
	if len(currentSpecs) == 0 {
		_ = transaction.Advance(&tx, transaction.Failed)
		writeAdminConfigVaultRestoreFailure(w, target.ID, "", tx, false, false, "precheck",
			errors.New("restore requires at least one current managed config file for a safety snapshot"))
		return
	}

	safetySnapshot, err := store.Capture(configvault.CaptureRequest{
		Component:     "admin",
		Reason:        "pre-restore safety snapshot for " + target.ID,
		TransactionID: transactionID,
		Artifacts:     currentSpecs,
	})
	if err != nil {
		_ = transaction.Advance(&tx, transaction.Failed)
		writeAdminConfigVaultRestoreFailure(w, target.ID, "", tx, false, false, "safety-snapshot", err)
		return
	}
	if err := transaction.Advance(&tx, transaction.Snapshot); err != nil {
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, false, false, "transaction", err)
		return
	}

	if err := validateAdminConfigVaultRestoreContent(store, adminConfigVaultManagedRoot, target); err != nil {
		_ = transaction.Advance(&tx, transaction.Failed)
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, false, false, "validation", err)
		return
	}
	if err := transaction.Advance(&tx, transaction.Validated); err != nil {
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, false, false, "transaction", err)
		return
	}
	if err := transaction.Advance(&tx, transaction.Applied); err != nil {
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, false, false, "transaction", err)
		return
	}

	if err := applyAdminConfigVaultSnapshot(store, adminConfigVaultManagedRoot, target); err != nil {
		rollbackOK, rollbackErr := rollbackAdminConfigVaultSnapshot(store, adminConfigVaultManagedRoot, safetySnapshot)
		if rollbackOK {
			_ = transaction.Advance(&tx, transaction.RolledBack)
		} else {
			_ = transaction.Advance(&tx, transaction.Ambiguous)
		}
		if rollbackErr != nil {
			err = fmt.Errorf("%v; rollback failed: %w", err, rollbackErr)
		}
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, rollbackOK, rollbackOK, "apply", err)
		return
	}

	diff, err := buildAdminConfigVaultDiff(target.ID)
	if err != nil || diff.Changed {
		verifyErr := err
		if verifyErr == nil {
			verifyErr = errors.New("post-restore readback does not match target snapshot")
		}
		rollbackOK, rollbackErr := rollbackAdminConfigVaultSnapshot(store, adminConfigVaultManagedRoot, safetySnapshot)
		if rollbackOK {
			_ = transaction.Advance(&tx, transaction.RolledBack)
		} else {
			_ = transaction.Advance(&tx, transaction.Ambiguous)
		}
		if rollbackErr != nil {
			verifyErr = fmt.Errorf("%v; rollback failed: %w", verifyErr, rollbackErr)
		}
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, rollbackOK, rollbackOK, "readback", verifyErr)
		return
	}

	if err := transaction.Advance(&tx, transaction.Verified); err != nil {
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, false, false, "transaction", err)
		return
	}
	if err := transaction.Advance(&tx, transaction.Committed); err != nil {
		writeAdminConfigVaultRestoreFailure(w, target.ID, safetySnapshot.ID, tx, false, false, "transaction", err)
		return
	}

	writeJSON(w, http.StatusOK, adminConfigVaultRestoreOutcome{
		OK:               true,
		TargetSnapshot:   target.ID,
		SafetySnapshot:   safetySnapshot.ID,
		Transaction:      tx,
		ReadbackVerified: true,
		Probe:            "managed-config-sha256-readback",
	})
}

func writeAdminConfigVaultRestoreFailure(
	w http.ResponseWriter,
	targetID string,
	safetyID string,
	tx transaction.Manifest,
	rolledBack bool,
	rollbackVerified bool,
	stage string,
	err error,
) {
	writeJSON(w, http.StatusInternalServerError, adminConfigVaultRestoreOutcome{
		OK:               false,
		TargetSnapshot:   targetID,
		SafetySnapshot:   safetyID,
		Transaction:      tx,
		RolledBack:       rolledBack,
		RollbackVerified: rollbackVerified,
		Probe:            stage,
		Error:            err.Error(),
	})
}

func validateAdminConfigVaultRestoreManifest(root string, manifest configvault.Manifest) error {
	if manifest.SchemaVersion != configvault.SchemaVersion {
		return errors.New("unsupported config vault schema")
	}
	if manifest.Component != "admin" {
		return errors.New("snapshot component must be admin")
	}
	if len(manifest.Artifacts) == 0 {
		return errors.New("snapshot contains no artifacts")
	}
	if len(manifest.Artifacts) > adminConfigVaultMaxFiles {
		return fmt.Errorf("snapshot contains more than %d artifacts", adminConfigVaultMaxFiles)
	}

	paths := make(map[string]struct{}, len(manifest.Artifacts))
	ids := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		if artifact.ID == "" {
			return errors.New("snapshot contains empty artifact id")
		}
		if _, exists := ids[artifact.ID]; exists {
			return fmt.Errorf("duplicate artifact id %q", artifact.ID)
		}
		ids[artifact.ID] = struct{}{}

		clean, err := validateAdminConfigVaultRestorePath(root, artifact.SourcePath)
		if err != nil {
			return fmt.Errorf("artifact %s: %w", artifact.ID, err)
		}
		if clean != artifact.SourcePath {
			return fmt.Errorf("artifact %s source path is not canonical", artifact.ID)
		}
		if _, exists := paths[clean]; exists {
			return fmt.Errorf("duplicate artifact path %q", clean)
		}
		paths[clean] = struct{}{}
	}
	return nil
}

func validateAdminConfigVaultRestoreContent(store *configvault.Store, root string, manifest configvault.Manifest) error {
	if err := validateAdminConfigVaultRestoreManifest(root, manifest); err != nil {
		return err
	}
	for _, artifact := range manifest.Artifacts {
		record, content, err := store.ReadArtifact(manifest.ID, artifact.ID)
		if err != nil {
			return fmt.Errorf("artifact %s: %w", artifact.ID, err)
		}
		if record.SourcePath != artifact.SourcePath || record.SHA256 != artifact.SHA256 || int64(len(content)) != artifact.Size {
			return fmt.Errorf("artifact %s manifest/object mismatch", artifact.ID)
		}
	}
	return nil
}

func validateAdminConfigVaultRestorePath(root, path string) (string, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	path = filepath.Clean(strings.TrimSpace(path))
	if !filepath.IsAbs(root) || !filepath.IsAbs(path) {
		return "", errors.New("restore paths must be absolute")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", errors.New("restore path is outside managed root")
	}
	return path, nil
}

func ensureAdminConfigVaultParent(root, target string) error {
	clean, err := validateAdminConfigVaultRestorePath(root, target)
	if err != nil {
		return err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("managed root must be a non-symlink directory")
	}

	relative, err := filepath.Rel(root, filepath.Dir(clean))
	if err != nil {
		return err
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, part := range strings.Split(relative, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err := os.Mkdir(current, 0700); err != nil {
				return err
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("restore parent is not a safe directory: %s", current)
		}
	}
	return nil
}

func applyAdminConfigVaultSnapshot(store *configvault.Store, root string, manifest configvault.Manifest) error {
	if err := validateAdminConfigVaultRestoreContent(store, root, manifest); err != nil {
		return err
	}

	artifacts := append([]configvault.ArtifactRecord(nil), manifest.Artifacts...)
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].SourcePath < artifacts[j].SourcePath })
	targets := make(map[string]struct{}, len(artifacts))

	for _, artifact := range artifacts {
		targets[artifact.SourcePath] = struct{}{}
		record, content, err := store.ReadArtifact(manifest.ID, artifact.ID)
		if err != nil {
			return err
		}
		if err := ensureAdminConfigVaultParent(root, record.SourcePath); err != nil {
			return err
		}
		if info, err := os.Lstat(record.SourcePath); err == nil {
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("restore target is not a regular non-symlink file: %s", record.SourcePath)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		mode := os.FileMode(record.Mode & 0777)
		if mode == 0 {
			mode = 0600
		}
		if err := safety.WriteFileAtomic(record.SourcePath, content, mode); err != nil {
			return err
		}
	}

	current, _, err := discoverAdminConfigVaultArtifacts()
	if err != nil {
		return err
	}
	for _, artifact := range current {
		if _, keep := targets[artifact.Path]; keep {
			continue
		}
		info, err := os.Lstat(artifact.Path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to remove non-regular managed path: %s", artifact.Path)
		}
		if err := os.Remove(artifact.Path); err != nil {
			return err
		}
	}
	return nil
}

func rollbackAdminConfigVaultSnapshot(
	store *configvault.Store,
	root string,
	safetySnapshot configvault.Manifest,
) (bool, error) {
	if err := applyAdminConfigVaultSnapshot(store, root, safetySnapshot); err != nil {
		return false, err
	}
	diff, err := buildAdminConfigVaultDiff(safetySnapshot.ID)
	if err != nil {
		return false, err
	}
	if diff.Changed {
		return false, errors.New("rollback readback does not match safety snapshot")
	}
	return true, nil
}
