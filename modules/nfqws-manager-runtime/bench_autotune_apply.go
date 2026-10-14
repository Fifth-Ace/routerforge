package main

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const benchAutoTuneApplyConfirm = "ROUTERFORGE_AUTOTUNE_APPLY"

type benchAutoTuneApplyRequest struct {
	ReceiptToken            string `json:"receipt_token"`
	ExpectedActiveSHA256    string `json:"expected_active_sha256"`
	ExpectedCandidateSHA256 string `json:"expected_candidate_sha256"`
	Confirm                 string `json:"confirm"`
}

type benchAutoTuneApplyReceipt struct {
	Token                string
	ActiveConfigSHA256   string
	CandidateSHA256      string
	CandidateConfig      string
	ServerName           string
	SourceProfileIndex   int
	AppendProfile        bool
	ListAppendName       string
	ListAppendSHA        string
	ListAppendBytes      []byte
	Transport            string
	SessionID            string
	CandidateID          string
	CandidateName        string
	CandidateSource      string
	CandidateFingerprint string
	LiveResultClass      string
	LiveSuccessRate      float64
	LiveCompleteRate     float64
	ExpectedLists        map[string]string
	ExpectedBlobs        map[string]string
	ExpiresAt            time.Time
}

var benchAutoTuneApplyReceiptState = struct {
	sync.Mutex
	receipt *benchAutoTuneApplyReceipt
}{}

func registerBenchAutoTuneApplyRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-autotune/apply", mutationOnly(handleBenchAutoTuneApply))
}

func clearBenchAutoTuneApplyReceipt() {
	benchAutoTuneApplyReceiptState.Lock()
	benchAutoTuneApplyReceiptState.receipt = nil
	benchAutoTuneApplyReceiptState.Unlock()
}

func storeBenchAutoTuneApplyReceipt(plan *benchAutoTuneApplyPlan, candidateConfig, candidateSHA string) error {
	return storeBenchAutoTuneApplyReceiptWithResources(plan, candidateConfig, candidateSHA, nil, nil)
}

func storeBenchAutoTuneApplyReceiptWithResources(
	plan *benchAutoTuneApplyPlan, candidateConfig, candidateSHA string,
	expectedLists, expectedBlobs map[string]string,
) error {
	if plan == nil || strings.TrimSpace(plan.Token) == "" {
		return errors.New("AutoTune apply plan is required")
	}
	if !smartApplyHashPattern.MatchString(plan.ConfigSHA256) ||
		!smartApplyHashPattern.MatchString(candidateSHA) {
		return errors.New("AutoTune apply receipt hashes must be SHA256")
	}
	if candidateConfig == "" {
		return errors.New("AutoTune candidate config must not be empty")
	}
	receipt := &benchAutoTuneApplyReceipt{
		Token:                plan.Token,
		ActiveConfigSHA256:   strings.ToLower(plan.ConfigSHA256),
		CandidateSHA256:      strings.ToLower(candidateSHA),
		CandidateConfig:      candidateConfig,
		ServerName:           plan.ServerName,
		SourceProfileIndex:   plan.SourceProfileIndex,
		AppendProfile:        plan.AppendProfile,
		ListAppendName:       plan.ListAppendName,
		ListAppendSHA:        plan.ListAppendSHA,
		Transport:            plan.Transport,
		SessionID:            plan.SessionID,
		CandidateID:          plan.CandidateID,
		CandidateName:        plan.CandidateName,
		CandidateSource:      plan.CandidateSource,
		CandidateFingerprint: plan.CandidateFingerprint,
		LiveResultClass:      plan.LiveResultClass,
		LiveSuccessRate:      plan.LiveSuccessRate,
		LiveCompleteRate:     plan.LiveCompleteRate,
		ExpectedLists:        v2CopyStringMap(expectedLists),
		ExpectedBlobs:        v2CopyStringMap(expectedBlobs),
		ExpiresAt:            plan.ExpiresAt,
	}
	benchAutoTuneApplyReceiptState.Lock()
	benchAutoTuneApplyReceiptState.receipt = receipt
	benchAutoTuneApplyReceiptState.Unlock()
	return nil
}

func currentBenchAutoTuneApplyReceipt(token string) (*benchAutoTuneApplyReceipt, error) {
	benchAutoTuneApplyReceiptState.Lock()
	receipt := benchAutoTuneApplyReceiptState.receipt
	if receipt == nil {
		benchAutoTuneApplyReceiptState.Unlock()
		return nil, errors.New("no active AutoTune preview receipt")
	}
	copyReceipt := *receipt
	copyReceipt.ExpectedLists = v2CopyStringMap(receipt.ExpectedLists)
	copyReceipt.ExpectedBlobs = v2CopyStringMap(receipt.ExpectedBlobs)
	copyReceipt.ListAppendBytes = append([]byte(nil), receipt.ListAppendBytes...)
	benchAutoTuneApplyReceiptState.Unlock()

	if time.Now().UTC().After(copyReceipt.ExpiresAt) {
		clearBenchAutoTuneApplyReceipt()
		return nil, errors.New("AutoTune preview receipt expired")
	}
	if subtle.ConstantTimeCompare([]byte(token), []byte(copyReceipt.Token)) != 1 {
		return nil, errors.New("AutoTune apply receipt token mismatch")
	}

	plan, err := currentBenchAutoTuneApplyPlan()
	if err != nil {
		clearBenchAutoTuneApplyReceipt()
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(plan.Token), []byte(copyReceipt.Token)) != 1 ||
		!strings.EqualFold(plan.ConfigSHA256, copyReceipt.ActiveConfigSHA256) ||
		plan.ServerName != copyReceipt.ServerName ||
		plan.SourceProfileIndex != copyReceipt.SourceProfileIndex ||
		plan.AppendProfile != copyReceipt.AppendProfile ||
		plan.ListAppendName != copyReceipt.ListAppendName ||
		plan.ListAppendSHA != copyReceipt.ListAppendSHA ||
		plan.SessionID != copyReceipt.SessionID ||
		plan.Transport != copyReceipt.Transport ||
		plan.CandidateSource != copyReceipt.CandidateSource ||
		plan.CandidateFingerprint != copyReceipt.CandidateFingerprint {
		clearBenchAutoTuneApplyReceipt()
		return nil, errors.New("AutoTune preview receipt no longer matches the active recommendation")
	}
	return &copyReceipt, nil
}

func consumeBenchAutoTuneApplyReceipt() {
	clearBenchAutoTuneApplyReceipt()
	clearBenchAutoTuneApplyPlan()
}

type autoTuneSmartApplyCapture struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *autoTuneSmartApplyCapture) Header() http.Header {
	if c.header == nil {
		c.header = make(http.Header)
	}
	return c.header
}

func (c *autoTuneSmartApplyCapture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

func (c *autoTuneSmartApplyCapture) Write(data []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return c.body.Write(data)
}

func executeAutoTuneThroughSmartApply(request smartApplyRequest) (int, map[string]any, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return 0, nil, err
	}
	inner, err := http.NewRequest(http.MethodPost, "/v1/smart-apply", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	capture := &autoTuneSmartApplyCapture{}
	handleSmartApply(capture, inner)
	status := capture.status
	if status == 0 {
		status = http.StatusInternalServerError
	}
	result := map[string]any{}
	if err := json.Unmarshal(capture.body.Bytes(), &result); err != nil {
		return status, nil, errors.New("decode Smart Apply result: " + err.Error())
	}
	return status, result, nil
}

func validateBenchAutoTuneApplyRequest(request benchAutoTuneApplyRequest) error {
	if request.Confirm != benchAutoTuneApplyConfirm {
		return errors.New("confirm must equal " + benchAutoTuneApplyConfirm)
	}
	if strings.TrimSpace(request.ReceiptToken) == "" {
		return errors.New("receipt_token is required")
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedActiveSHA256)) ||
		!smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedCandidateSHA256)) {
		return errors.New("expected AutoTune apply hashes must be SHA256")
	}
	return nil
}

func handleBenchAutoTuneApply(w http.ResponseWriter, r *http.Request) {
	var request benchAutoTuneApplyRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid AutoTune apply request"})
		return
	}
	if err := validateBenchAutoTuneApplyRequest(request); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}

	receipt, err := currentBenchAutoTuneApplyReceipt(request.ReceiptToken)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(request.ExpectedActiveSHA256), receipt.ActiveConfigSHA256) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "AutoTune apply active config identity mismatch"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(request.ExpectedCandidateSHA256), receipt.CandidateSHA256) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "AutoTune apply candidate config identity mismatch"})
		return
	}

	plan, err := currentBenchAutoTuneApplyPlan()
	if err != nil {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	// An append plan intentionally has no production source profile. Replacement
	// plans still require the exact eligible source and matching strategy args.
	var sourceArgs []string
	if !plan.AppendProfile {
		inventory := readBenchStrategyInventory()
		var source *benchStrategyProfile
		for i := range inventory.Profiles {
			if inventory.Profiles[i].Index == plan.SourceProfileIndex {
				source = &inventory.Profiles[i]
				break
			}
		}
		if source == nil || !v2ProductionSourceProfileEligible(*source) {
			clearBenchAutoTuneApplyReceipt()
			writeJSON(w, http.StatusConflict, map[string]any{"error": "tested source profile identity is no longer eligible"})
			return
		}
		sourceArgs = plan.SourceStrategyArgs
		if len(sourceArgs) == 0 {
			sourceArgs = source.Args
		}
		if !stringSlicesEqual(source.Args, sourceArgs) {
			clearBenchAutoTuneApplyReceipt()
			writeJSON(w, http.StatusConflict, map[string]any{"error": "source production profile drifted since AutoTune preview"})
			return
		}
	}
	candidateArgs := plan.CandidateStrategyArgs
	if len(candidateArgs) == 0 {
		candidateArgs = plan.StrategyArgs
	}
	if err := validatePreviewArgs(candidateArgs); err != nil {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "candidate strategy identity is invalid: " + err.Error()})
		return
	}
	fingerprint := v2CandidateTechniqueFingerprint(candidateArgs)
	if fingerprint == "" || (plan.CandidateFingerprint != "" && fingerprint != plan.CandidateFingerprint) {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "candidate technique fingerprint drifted since AutoTune preview"})
		return
	}

	status := readStatus()
	if !strings.EqualFold(status.ConfigSHA256, receipt.ActiveConfigSHA256) {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "active config changed since AutoTune preview",
			"current_sha256": status.ConfigSHA256,
		})
		return
	}
	configData, err := os.ReadFile(configPath)
	if err != nil {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read active config before AutoTune apply: " + err.Error()})
		return
	}
	if len(configData) > configMaxBytes {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "active config exceeds AutoTune apply safety limit"})
		return
	}
	fileHash := smartApplySHA256(configData)
	if !strings.EqualFold(fileHash, receipt.ActiveConfigSHA256) {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "active config file changed since AutoTune preview",
			"current_sha256": fileHash,
		})
		return
	}
	var candidate string
	if plan.ListAppendName != "" {
		candidate = string(configData) // the verified technique and active config remain unchanged
	} else {
		candidate, err = nf4BuildPlacementConfig(string(configData), plan.AppendProfile, sourceArgs, candidateArgs)
	}
	if err != nil {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "rebuild deterministic AutoTune candidate: " + err.Error()})
		return
	}
	candidateSHA := smartApplySHA256([]byte(candidate))
	if !strings.EqualFold(candidateSHA, receipt.CandidateSHA256) ||
		candidate != receipt.CandidateConfig {
		clearBenchAutoTuneApplyReceipt()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "AutoTune candidate drifted since preview"})
		return
	}

	smartRequest := smartApplyRequest{
		Config:               candidate,
		ExpectedActiveSHA256: receipt.ActiveConfigSHA256,
		PostVerifyTarget:     receipt.ServerName,
		PostVerifyIPv4:       plan.DestinationIPv4,
		ExpectedConfigSHA256: receipt.CandidateSHA256,
		ExpectedLists:        v2CopyStringMap(receipt.ExpectedLists),
		ExpectedBlobs:        v2CopyStringMap(receipt.ExpectedBlobs),
		Confirm:              "NFQWS_SMART_APPLY",
	}
	if plan.ListAppendName != "" {
		_, name, before, updated, placementErr := nf4bFindReusableList(
			plan.ServerName, plan.DestinationIPv4, plan.CandidateFingerprint, readBenchStrategyInventory(),
		)
		if placementErr != nil || name != plan.ListAppendName || before != plan.ListAppendSHA ||
			receipt.ListAppendName != name || receipt.ListAppendSHA != before ||
			!bytes.Equal(updated, receipt.ListAppendBytes) {
			clearBenchAutoTuneApplyReceipt()
			writeJSON(w, http.StatusConflict, map[string]any{"error": "NF4B list changed or exclusive placement proof expired"})
			return
		}
		smartRequest.Lists = []smartApplyDependency{{Name: name,
			ContentBase64: base64.StdEncoding.EncodeToString(updated), ExpectedSHA256: listSourceSHA256(updated)}}
		smartRequest.ExpectedExistingLists = map[string]string{name: before}
		if smartRequest.ExpectedLists == nil {
			smartRequest.ExpectedLists = make(map[string]string)
		}
		smartRequest.ExpectedLists[name] = listSourceSHA256(updated)
	}

	// One-shot: invalidate the receipt and recommendation before mutation.
	// Any failure requires a fresh AutoTune + preview, preventing replay.
	consumeBenchAutoTuneApplyReceipt()

	code, result, err := executeAutoTuneThroughSmartApply(smartRequest)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error(), "receipt_consumed": true})
		return
	}
	result["autotune_apply"] = true
	result["post_apply_target"] = receipt.ServerName
	result["append_profile"] = receipt.AppendProfile
	result["updated_existing_list"] = receipt.ListAppendName != ""
	result["updated_list_name"] = receipt.ListAppendName
	result["receipt_consumed"] = true
	result["server_name"] = receipt.ServerName
	result["source_profile_index"] = receipt.SourceProfileIndex
	result["candidate_source"] = receipt.CandidateSource
	result["candidate_id"] = receipt.CandidateID
	result["candidate_name"] = receipt.CandidateName
	result["candidate_fingerprint"] = receipt.CandidateFingerprint
	result["session_id"] = receipt.SessionID
	result["transport"] = receipt.Transport
	result["live_result_class"] = receipt.LiveResultClass
	result["preview_candidate_sha256"] = receipt.CandidateSHA256
	writeJSON(w, code, result)
}
