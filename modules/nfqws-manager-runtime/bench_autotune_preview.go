package main

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const benchAutoTunePreviewConfirm = "ROUTERFORGE_AUTOTUNE_PREVIEW"

type benchAutoTunePreviewRequest struct {
	Token                string `json:"token"`
	ExpectedConfigSHA256 string `json:"expected_config_sha256"`
	Confirm              string `json:"confirm"`
}

type benchAutoTunePreviewResponse struct {
	OK                    bool     `json:"ok"`
	PreviewOnly           bool     `json:"preview_only"`
	ApplyEnabled          bool     `json:"apply_enabled"`
	ServerName            string   `json:"server_name"`
	DestinationIPv4       string   `json:"destination_ipv4"`
	SourceProfileIndex    int      `json:"source_profile_index"`
	ActiveConfigSHA256    string   `json:"active_config_sha256"`
	CandidateConfigSHA256 string   `json:"candidate_config_sha256"`
	CandidateConfig       string   `json:"candidate_config"`
	Changed               bool     `json:"changed"`
	SourceStrategyArgs    []string `json:"source_strategy_args"`
	CandidateStrategyArgs []string `json:"candidate_strategy_args"`
	GateExpiresAt         string   `json:"gate_expires_at"`
	Transport             string   `json:"transport,omitempty"`
	SessionID             string   `json:"session_id,omitempty"`
	CandidateID           string   `json:"candidate_id,omitempty"`
	CandidateName         string   `json:"candidate_name,omitempty"`
	CandidateSource       string   `json:"candidate_source,omitempty"`
	CandidateFingerprint  string   `json:"candidate_fingerprint,omitempty"`
	LiveResultClass       string   `json:"live_result_class,omitempty"`
	LiveSuccessRate       float64  `json:"live_success_rate,omitempty"`
	LiveCompleteRate      float64  `json:"live_complete_rate,omitempty"`
	RequiredResources     []string `json:"required_resources"`
	ReloadMethod          string   `json:"reload_method"`
	Warnings              []string `json:"warnings"`
	Reason                string   `json:"reason"`
}

type configTokenOffset struct {
	Value string
	Start int
	End   int
}

func registerBenchAutoTuneApplyPreviewRoute(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-autotune/apply-preview", mutationOnly(handleBenchAutoTuneApplyPreview))
	registerBenchAutoTuneApplyRoute(mux)
}

func configSpaceAt(text string, offset int) (bool, int) {
	r, size := utf8.DecodeRuneInString(text[offset:])
	if r == utf8.RuneError && size == 0 {
		return false, 0
	}
	return unicode.IsSpace(r), size
}

func splitConfigTokenOffsets(text string) []configTokenOffset {
	out := []configTokenOffset{}
	for i := 0; i < len(text); {
		space, size := configSpaceAt(text, i)
		if space {
			i += size
			continue
		}
		start := i
		for i < len(text) {
			space, size = configSpaceAt(text, i)
			if space {
				break
			}
			i += size
		}
		out = append(out, configTokenOffset{Value: text[start:i], Start: start, End: i})
	}
	return out
}

func quotedAssignmentValue(config, name string) (int, int, string, error) {
	prefix := name + "=\""
	matches := []int{}
	lineStart := 0
	for lineStart < len(config) {
		lineEnd := strings.IndexByte(config[lineStart:], '\n')
		if lineEnd < 0 {
			lineEnd = len(config)
		} else {
			lineEnd += lineStart
		}
		line := config[lineStart:lineEnd]
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, prefix) {
			matches = append(matches, lineStart+len(line)-len(trimmed)+len(prefix))
		}
		if lineEnd == len(config) {
			break
		}
		lineStart = lineEnd + 1
	}
	if len(matches) != 1 {
		return 0, 0, "", errors.New("NFQWS_ARGS_CUSTOM assignment must appear exactly once")
	}
	valueStart := matches[0]
	escaped := false
	for i := valueStart; i < len(config); i++ {
		if config[i] == '\\' {
			escaped = !escaped
			continue
		}
		if config[i] == '"' && !escaped {
			lineEnd := strings.IndexByte(config[i:], '\n')
			if lineEnd < 0 {
				lineEnd = len(config)
			} else {
				lineEnd += i
			}
			if strings.TrimSpace(config[i+1:lineEnd]) != "" {
				return 0, 0, "", errors.New("NFQWS_ARGS_CUSTOM closing quote must terminate the assignment line")
			}
			return valueStart, i, config[valueStart:i], nil
		}
		escaped = false
	}
	return 0, 0, "", errors.New("NFQWS_ARGS_CUSTOM closing quote not found")
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func validatePreviewArgs(args []string) error {
	if len(args) == 0 {
		return errors.New("strategy args must not be empty")
	}
	for _, arg := range args {
		if arg == "" || strings.IndexFunc(arg, unicode.IsSpace) >= 0 || arg == "--new" {
			return errors.New("strategy args contain an unsupported token")
		}
	}
	return nil
}

func buildAutoTuneCandidateConfig(config string, sourceArgs, candidateArgs []string) (string, error) {
	if err := validatePreviewArgs(sourceArgs); err != nil {
		return "", err
	}
	if err := validatePreviewArgs(candidateArgs); err != nil {
		return "", err
	}
	valueStart, _, body, err := quotedAssignmentValue(config, "NFQWS_ARGS_CUSTOM")
	if err != nil {
		return "", err
	}
	tokens := splitConfigTokenOffsets(body)
	matches := []int{}
	for i := 0; i+len(sourceArgs) <= len(tokens); i++ {
		ok := true
		for j := range sourceArgs {
			if tokens[i+j].Value != sourceArgs[j] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		beforeOK := i == 0 || tokens[i-1].Value == "--new"
		after := i + len(sourceArgs)
		afterOK := after == len(tokens) || tokens[after].Value == "--new"
		if beforeOK && afterOK {
			matches = append(matches, i)
		}
	}
	if len(matches) != 1 {
		return "", errors.New("tested source profile must match exactly one complete NFQWS_ARGS_CUSTOM profile")
	}
	first := matches[0]
	last := first + len(sourceArgs) - 1
	start := valueStart + tokens[first].Start
	end := valueStart + tokens[last].End
	candidate := config[:start] + strings.Join(candidateArgs, " ") + config[end:]
	if err := validateConfig(candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func handleBenchAutoTuneApplyPreview(w http.ResponseWriter, r *http.Request) {
	var request benchAutoTunePreviewRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid AutoTune preview request"})
		return
	}
	if request.Confirm != benchAutoTunePreviewConfirm {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal " + benchAutoTunePreviewConfirm})
		return
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(request.ExpectedConfigSHA256)) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "expected_config_sha256 must be SHA256"})
		return
	}
	plan, err := currentBenchAutoTuneApplyPlan()
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error()})
		return
	}
	if subtle.ConstantTimeCompare([]byte(request.Token), []byte(plan.Token)) != 1 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "AutoTune preview token mismatch"})
		return
	}
	if !strings.EqualFold(strings.TrimSpace(request.ExpectedConfigSHA256), plan.ConfigSHA256) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "AutoTune preview config identity mismatch"})
		return
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "read active config: " + err.Error()})
		return
	}
	if len(configData) > configMaxBytes {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "active config exceeds preview safety limit"})
		return
	}
	activeHash := smartApplySHA256(configData)
	if !strings.EqualFold(activeHash, plan.ConfigSHA256) {
		clearBenchAutoTuneApplyPlan()
		writeJSON(w, http.StatusConflict, map[string]any{"error": "active config changed since AutoTune recommendation"})
		return
	}

	inventory := readBenchStrategyInventory()
	var source *benchStrategyProfile
	for i := range inventory.Profiles {
		if inventory.Profiles[i].Index == plan.SourceProfileIndex {
			source = &inventory.Profiles[i]
			break
		}
	}
	if source == nil || !v2ProductionSourceProfileEligible(*source) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "tested source profile identity is no longer eligible"})
		return
	}
	sourceArgs := plan.SourceStrategyArgs
	if len(sourceArgs) == 0 {
		sourceArgs = source.Args
	}
	if !stringSlicesEqual(source.Args, sourceArgs) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "source production profile drifted since AutoTune recommendation"})
		return
	}
	candidateArgs := plan.CandidateStrategyArgs
	if len(candidateArgs) == 0 {
		candidateArgs = plan.StrategyArgs
	}
	if err := validatePreviewArgs(candidateArgs); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "candidate strategy identity is invalid: " + err.Error()})
		return
	}
	fingerprint := v2CandidateTechniqueFingerprint(candidateArgs)
	if fingerprint == "" || (plan.CandidateFingerprint != "" && fingerprint != plan.CandidateFingerprint) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "candidate technique fingerprint drifted since live verification"})
		return
	}
	if _, err := v2CustomProfile(v2PortableCandidateArgs(candidateArgs), plan.ServerName); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "candidate technique no longer compiles: " + err.Error()})
		return
	}

	candidateConfig, err := buildAutoTuneCandidateConfig(string(configData), source.Args, candidateArgs)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "build deterministic AutoTune candidate: " + err.Error()})
		return
	}
	candidateHash := smartApplySHA256([]byte(candidateConfig))
	expectedLists, expectedBlobs, resources, err := v2SnapshotCandidateDependencies(candidateConfig)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "snapshot candidate dependencies: " + err.Error()})
		return
	}
	if err := storeBenchAutoTuneApplyReceiptWithResources(plan, candidateConfig, candidateHash, expectedLists, expectedBlobs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "create AutoTune preview receipt: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, benchAutoTunePreviewResponse{
		OK:                    true,
		PreviewOnly:           true,
		ApplyEnabled:          false,
		ServerName:            plan.ServerName,
		DestinationIPv4:       plan.DestinationIPv4,
		SourceProfileIndex:    plan.SourceProfileIndex,
		ActiveConfigSHA256:    activeHash,
		CandidateConfigSHA256: candidateHash,
		CandidateConfig:       candidateConfig,
		Changed:               candidateHash != activeHash,
		SourceStrategyArgs:    append([]string{}, source.Args...),
		CandidateStrategyArgs: append([]string{}, candidateArgs...),
		GateExpiresAt:        plan.ExpiresAt.Format(time.RFC3339),
		Transport:            plan.Transport,
		SessionID:            plan.SessionID,
		CandidateID:          plan.CandidateID,
		CandidateName:        plan.CandidateName,
		CandidateSource:      plan.CandidateSource,
		CandidateFingerprint: plan.CandidateFingerprint,
		LiveResultClass:      plan.LiveResultClass,
		LiveSuccessRate:      plan.LiveSuccessRate,
		LiveCompleteRate:     plan.LiveCompleteRate,
		RequiredResources:    resources,
		ReloadMethod:         "Smart Apply: controlled restart when service is running; no restart when stopped",
		Warnings:             []string{},
		Reason:               "deterministic preview only; production Apply remains locked",
	})
}
