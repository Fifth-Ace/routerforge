//go:build linux

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"runtime"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/apptransaction"
)

// The new planner and admission guard are scoped to the Dev ARM64 build.
// No HTTP handler launches opkg yet. The worker must not become executable
// until all cross-module mutation paths and per-package health checks agree.
type coreTransactionState struct {
	enabled     bool
	coordinator *apptransaction.Coordinator
	err         error
}

var coreTransactionIntegration coreTransactionState

func initCoreAppTransactions() {
	if runtime.GOARCH != "arm64" || !strings.HasPrefix(version, "1~dev.r") {
		return
	}
	coreTransactionIntegration.enabled = true
	coreTransactionIntegration.coordinator, coreTransactionIntegration.err = apptransaction.DeviceCoordinator()
	if coreTransactionIntegration.err != nil {
		log.Printf("RF_APP_TRANSACTION: admission unavailable: %v", coreTransactionIntegration.err)
	}
}

// The existing TCP listener must bind successfully before exposing a local
// lifecycle candidate. The evidence socket is not a public route or worker API.
func serveCoreHTTPWithEvidence(server *http.Server, build string) error {
	if !coreTransactionIntegration.enabled || coreTransactionIntegration.coordinator == nil {
		return server.ListenAndServe()
	}
	addr := server.Addr
	if addr == "" {
		addr = ":http"
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	evidence, err := apptransaction.StartCoreLifecycleEvidence(build, profilingStatusSnapshot)
	if err != nil {
		log.Printf("RF_APP_TRANSACTION: native core evidence unavailable: %v", err)
	} else {
		defer evidence.Close()
	}
	return server.Serve(listener)
}

func acquireCorePackageMutation() (func(), error) {
	return coreTransactionIntegration.acquire()
}
func (s coreTransactionState) acquire() (func(), error) {
	if !s.enabled {
		return func() {}, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	if s.coordinator == nil {
		return nil, errors.New("transaction admission unavailable")
	}
	f, err := s.coordinator.AcquireLegacy()
	if err != nil {
		return nil, err
	}
	return func() { _ = f.Close() }, nil
}
func coreMutationHTTPStatus(err error) int {
	if errors.Is(err, apptransaction.ErrBusy) || errors.Is(err, apptransaction.ErrRecovery) {
		return http.StatusConflict
	}
	return http.StatusServiceUnavailable
}

type appTransactionPreviewRequest struct {
	ExpectedCommit string   `json:"expected_commit"`
	Packages       []string `json:"packages"`
}
type appTransactionPreview struct {
	Plan               apptransaction.Plan `json:"plan"`
	PlanSHA256         string              `json:"plan_sha256"`
	DownloadBytes      int64               `json:"download_bytes"`
	ExecutionAvailable bool                `json:"execution_available"`
	Reason             string              `json:"reason"`
}

// Pure planner: compare the trusted remote release snapshot with the server's
// current catalog. Browser input contains only selected package names and the
// expected release commit; never URLs, hashes, commands or local paths.
func buildCoreTransactionPreview(id string, req appTransactionPreviewRequest, doc routerForgeReleaseIndex, status routerForgeReleaseStatus, items []catalogItem) (appTransactionPreview, error) {
	var out appTransactionPreview
	if !status.Online || status.Source != "remote" || !status.Supported || status.Channel != "dev" || status.Target != "aarch64-3.10" || doc.Commit != req.ExpectedCommit {
		return out, errors.New("fresh matching Dev ARM64 release required")
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return out, err
	}
	plan, err := apptransaction.FreezeDevPlan(data, id, req.ExpectedCommit, req.Packages)
	if err != nil {
		return out, err
	}
	for _, p := range plan.Packages {
		count := 0
		for _, item := range items {
			if routerForgePackageForItem(item) != p.Name {
				continue
			}
			count++
			if !item.Installed || !item.UpdateAvailable || !catalogActionAllowed(item, "update") || !officialRouterForgePackageAction(item, "update") || !packageManagementAllowsCatalogAction(item, "update") || appSourceActionBlockReason(item, "update", "") != "" {
				return out, fmt.Errorf("package not eligible: %s", p.Name)
			}
			if item.Version == p.Version || apptransaction.ValidateDevUpgrade(item.Version, p.Version) != nil {
				return out, fmt.Errorf("package is not a forward Dev upgrade: %s", p.Name)
			}
			rel := item.Release
			if rel.Package != p.Name || rel.Version != p.Version || rel.SHA256 != p.SHA256 || rel.Asset != p.Asset || rel.SizeBytes != uint64(p.Size) || rel.Architecture != plan.Target || rel.Channel != plan.Channel {
				return out, fmt.Errorf("catalog/release drift: %s", p.Name)
			}
		}
		if count != 1 {
			return out, fmt.Errorf("missing or ambiguous catalog package: %s", p.Name)
		}
		out.DownloadBytes += p.Size
	}
	h, err := plan.Hash()
	if err != nil {
		return out, err
	}
	out.Plan = plan
	out.PlanSHA256 = h
	out.ExecutionAvailable = false
	out.Reason = "executor activation gated pending global mutation and lifecycle acceptance"
	return out, nil
}

func decodeTransactionPreview(w http.ResponseWriter, r *http.Request) (appTransactionPreviewRequest, error) {
	var req appTransactionPreviewRequest
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	d.DisallowUnknownFields()
	if err := d.Decode(&req); err != nil {
		return req, err
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return req, errors.New("trailing request data")
	}
	return req, nil
}

func registerCoreAppTransactionHandlers(mux *http.ServeMux, auth *authManager) {
	authorized := func(r *http.Request) bool {
		if !auth.authRequired() {
			return true
		}
		u, ok := auth.sessionUser(r)
		return ok && u == "root"
	}
	mux.HandleFunc("/api/apps/transactions", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "root session required"})
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required; transaction execution not activated"})
			return
		}
		if !coreTransactionIntegration.enabled {
			writeCatalogJSON(w, http.StatusNotImplemented, map[string]any{"execution_available": false, "error": "Dev ARM64 transaction integration unavailable"})
			return
		}
		if coreTransactionIntegration.coordinator == nil {
			writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"execution_available": false, "error": "transaction admission unavailable"})
			return
		}
		status, err := coreTransactionIntegration.coordinator.Status()
		if err != nil {
			writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"execution_available": false, "state": status.State, "error": "transaction journal requires inspection"})
			return
		}
		writeCatalogJSON(w, http.StatusOK, map[string]any{"execution_available": false, "slot": status})
	})
	mux.HandleFunc("/api/apps/transactions/preview", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r) || !sameOriginRequest(r) {
			writeCatalogJSON(w, http.StatusForbidden, map[string]any{"error": "transaction preview not authorized"})
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
			return
		}
		if !coreTransactionIntegration.enabled || normalizedReleaseChannel() != "dev" || normalizedReleaseTarget() != "aarch64-3.10" {
			writeCatalogJSON(w, http.StatusNotImplemented, map[string]any{"execution_available": false, "error": "Dev ARM64 transaction integration unavailable"})
			return
		}
		if coreTransactionIntegration.coordinator == nil {
			writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"execution_available": false, "error": "transaction admission unavailable"})
			return
		}
		req, err := decodeTransactionPreview(w, r)
		if err != nil {
			writeCatalogJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid transaction preview request"})
			return
		}
		var nonce [16]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			writeCatalogJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "transaction identity unavailable"})
			return
		}
		doc, status := routerForgeReleaseSnapshot()
		out, err := buildCoreTransactionPreview(hex.EncodeToString(nonce[:]), req, doc, status, readCatalog().Modules)
		if err != nil {
			writeCatalogJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "execution_available": false})
			return
		}
		// Reject channel drift after reading the snapshot. No mutation takes place.
		if normalizedReleaseChannel() != out.Plan.Channel || normalizedReleaseTarget() != out.Plan.Target {
			writeCatalogJSON(w, http.StatusConflict, map[string]any{"error": "release selection changed"})
			return
		}
		writeCatalogJSON(w, http.StatusOK, out)
	})
}
