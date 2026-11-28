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
	"mime"
	"net"
	"net/http"
	"runtime"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/apptransaction"
)

// The new planner and admission guard are scoped to the Dev ARM64 build.
// Official Dev updates use the independent worker and shared package lease.
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
	f, err := s.coordinator.AcquirePackageMutation()
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
	Management         []apptransaction.ManagementPreview `json:"management,omitempty"`
	Plan               apptransaction.Plan                `json:"plan"`
	PlanSHA256         string                             `json:"plan_sha256"`
	DownloadBytes      int64                              `json:"download_bytes"`
	ExecutionAvailable bool                               `json:"execution_available"`
	Reason             string                             `json:"reason"`
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
	out.Reason = "update worker availability is checked by the HTTP handler"
	return out, nil
}

type coreTransactionEligiblePackage struct {
	Package          string `json:"package"`
	InstalledVersion string `json:"installed_version"`
	TargetVersion    string `json:"target_version"`
}

// Every advertised package must pass the same immutable-plan admission used
// for a user selection. The browser never guesses package IDs or release SHA.
func coreTransactionEligibleSelection(doc routerForgeReleaseIndex, status routerForgeReleaseStatus, items []catalogItem) (string, []coreTransactionEligiblePackage) {
	rows := make([]coreTransactionEligiblePackage, 0)
	if len(doc.Components) != 11 {
		return "", rows
	}
	for _, component := range doc.Components {
		req := appTransactionPreviewRequest{ExpectedCommit: doc.Commit, Packages: []string{component.Package}}
		if _, err := buildCoreTransactionPreview("selection", req, doc, status, items); err != nil {
			continue
		}
		for _, item := range items {
			if routerForgePackageForItem(item) == component.Package {
				rows = append(rows, coreTransactionEligiblePackage{component.Package, item.Version, component.Version})
				break
			}
		}
	}
	if len(rows) == 0 {
		return "", rows
	}
	return doc.Commit, rows
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

// Start accepts only the identity of a previously shown plan. The plan is
// rebuilt from trusted metadata and its exact hash must still match.
type appTransactionStartRequest struct {
	ID             string   `json:"id"`
	ExpectedCommit string   `json:"expected_commit"`
	Packages       []string `json:"packages"`
	PlanSHA256     string   `json:"plan_sha256"`
	Confirm        string   `json:"confirm"`
}

func exactLowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func decodeTransactionStart(w http.ResponseWriter, r *http.Request) (appTransactionStartRequest, error) {
	var out appTransactionStartRequest
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return out, errors.New("JSON required")
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return out, errors.New("object required")
	}
	seen := map[string]bool{}
	for d.More() {
		tok, err = d.Token()
		key, ok := tok.(string)
		if err != nil || !ok || seen[key] {
			return out, errors.New("duplicate or invalid field")
		}
		seen[key] = true
		switch key {
		case "id":
			err = d.Decode(&out.ID)
		case "expected_commit":
			err = d.Decode(&out.ExpectedCommit)
		case "packages":
			err = d.Decode(&out.Packages)
		case "plan_sha256":
			err = d.Decode(&out.PlanSHA256)
		case "confirm":
			err = d.Decode(&out.Confirm)
		default:
			return out, errors.New("unknown field")
		}
		if err != nil {
			return out, err
		}
	}
	tok, err = d.Token()
	if err != nil || tok != json.Delim('}') {
		return out, errors.New("invalid object")
	}
	if _, err = d.Token(); err != io.EOF {
		return out, errors.New("trailing request data")
	}
	if len(seen) != 5 || !exactLowerHex(out.ID, 32) || !exactLowerHex(out.ExpectedCommit, 40) || !exactLowerHex(out.PlanSHA256, 64) || out.Confirm != "EXECUTE:"+out.PlanSHA256 || len(out.Packages) < 1 || len(out.Packages) > 11 {
		return out, errors.New("invalid confirmed plan identity")
	}
	packages := map[string]bool{}
	for _, name := range out.Packages {
		if !strings.HasPrefix(name, "routerforge-") || len(name) > 80 || packages[name] {
			return out, errors.New("invalid package selection")
		}
		packages[name] = true
	}
	return out, nil
}
func confirmedTransactionPlan(req appTransactionStartRequest, doc routerForgeReleaseIndex, status routerForgeReleaseStatus, items []catalogItem) (apptransaction.Plan, error) {
	preview, err := buildCoreTransactionPreview(req.ID, appTransactionPreviewRequest{ExpectedCommit: req.ExpectedCommit, Packages: req.Packages}, doc, status, items)
	if err != nil {
		return apptransaction.Plan{}, err
	}
	if preview.PlanSHA256 != req.PlanSHA256 || req.Confirm != "EXECUTE:"+req.PlanSHA256 {
		return apptransaction.Plan{}, errors.New("confirmed plan changed")
	}
	return preview.Plan, nil
}
func coreExecutionPolicyActive() bool { return coreExecutionConfigured() }
func coreExecutionConfigured() bool {
	return apptransaction.DeviceUpdateAvailable(version) && coreTransactionIntegration.enabled && coreTransactionIntegration.coordinator != nil && normalizedReleaseChannel() == "dev" && normalizedReleaseTarget() == "aarch64-3.10"
}
func coreLegacyOfficialUpdateBlocked(item catalogItem, action string) bool {
	if !coreExecutionPolicyActive() || action != "update" {
		return false
	}
	if item.ID == "routerforge-core" {
		return true
	}
	return strings.HasPrefix(routerForgePackageForItem(item), "routerforge-") && strings.EqualFold(item.Publisher.ID, "routerforge") && strings.EqualFold(item.Trust.Status, "official")
}
func coreExecutionAuthorized(auth *authManager, r *http.Request) bool {
	if !auth.authRequired() {
		return true
	}
	u, ok := auth.sessionUser(r)
	return ok && u == "root"
}
func registerCoreTransactionStart(mux *http.ServeMux, auth *authManager) {
	mux.HandleFunc("/api/apps/transactions/start", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeCatalogJSON(w, 405, map[string]any{"error": "POST required"})
			return
		}
		if !coreExecutionAuthorized(auth, r) || strings.TrimSpace(r.Header.Get("Origin")) == "" || !sameOriginRequest(r) {
			writeCatalogJSON(w, 403, map[string]any{"error": "same-origin package update authorization required"})
			return
		}
		if !coreExecutionConfigured() {
			writeCatalogJSON(w, 409, map[string]any{"error": "update worker unavailable", "execution_available": false})
			return
		}
		req, err := decodeTransactionStart(w, r)
		if err != nil {
			writeCatalogJSON(w, 400, map[string]any{"error": "invalid confirmed plan"})
			return
		}
		doc, status := routerForgeReleaseSnapshot()
		plan, err := confirmedTransactionPlan(req, doc, status, readCatalog().Modules)
		if err != nil {
			writeCatalogJSON(w, 409, map[string]any{"error": "confirmed plan changed"})
			return
		}
		// Rebuild again inside the lease, before creating any execution slot. The
		// independent worker separately verifies the acceptance sequence and proofs.
		_, err = coreTransactionIntegration.coordinator.StartSimpleDeviceUpdate(r.Context(), version, plan, func() error {
			if !coreExecutionConfigured() || !coreExecutionAuthorized(auth, r) {
				return errors.New("execution admission changed")
			}
			doc, status := routerForgeReleaseSnapshot()
			_, err := confirmedTransactionPlan(req, doc, status, readCatalog().Modules)
			return err
		})
		if err != nil {
			log.Printf("RF_APP_TRANSACTION: start rejected: %v", err)
			writeCatalogJSON(w, 409, map[string]any{"error": "update not started: " + err.Error()})
			return
		}
		writeCatalogJSON(w, http.StatusAccepted, map[string]any{"id": plan.ID, "state": "queued"})
	})
}

func registerCoreAppTransactionHandlers(mux *http.ServeMux, auth *authManager) {
	registerCoreTransactionStart(mux, auth)
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
			writeCatalogJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
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
			status.State = string(apptransaction.RecoveryRequired)
		}
		if r.URL.Query().Get("progress") == "1" {
			history, historyErr := coreTransactionIntegration.coordinator.UpdateHistory(20)
			writeCatalogJSON(w, http.StatusOK, map[string]any{"slot": status, "history": history, "history_available": historyErr == nil, "log": coreTransactionIntegration.coordinator.UpdateLog(), "downloads": coreTransactionIntegration.coordinator.UpdateDownloads(status.Plan)})
			return
		}
		doc, releaseStatus := routerForgeReleaseSnapshot()
		commit, eligible := coreTransactionEligibleSelection(doc, releaseStatus, readCatalog().Modules)
		if normalizedReleaseChannel() != "dev" || normalizedReleaseTarget() != "aarch64-3.10" {
			commit = ""
			eligible = nil
		}
		history, historyErr := coreTransactionIntegration.coordinator.UpdateHistory(20)
		writeCatalogJSON(w, http.StatusOK, map[string]any{"execution_available": coreExecutionConfigured() && coreExecutionAuthorized(auth, r), "execution_policy_active": coreExecutionPolicyActive(), "slot": status, "release_commit": commit, "eligible_packages": eligible, "history": history, "history_available": historyErr == nil})
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
		out.ExecutionAvailable = coreExecutionConfigured() && coreExecutionAuthorized(auth, r)
		out.Reason = ""
		writeCatalogJSON(w, http.StatusOK, out)
	})
}
