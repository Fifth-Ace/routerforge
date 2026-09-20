package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	v2TargetMemoryVersion        = 1
	v2TargetMemoryFeatureVersion = 3
	v2TargetMemoryMax     = 512
	v2TargetMemoryConfirm = "ROUTERFORGE_V2_MEMORY_CLEAR"
)

var (
	v2TargetMemoryRoot   = "/opt/var/lib/routerforge/nfqws-manager"
	v2TargetMemoryPath   = v2TargetMemoryRoot + "/target-memory.json"
	v2TargetMemoryNow    = time.Now
	v2TargetMemoryBackup = recordPersistentBackupTarget
)

type v2TargetMemoryEntry struct {
	Target               string   `json:"target"`
	Protocol             string   `json:"protocol"`
	IPFamily             string   `json:"ip_family"`
	CandidateID          string   `json:"candidate_id"`
	CandidateName        string   `json:"candidate_name"`
	CandidateSource      string   `json:"candidate_source"`
	Fingerprint          string   `json:"fingerprint"`
	Environment          string   `json:"environment_fingerprint"`
	ConfigSHA256         string   `json:"config_sha256"`
	Args                 []string `json:"args"`
	ResultClass          string   `json:"result_class"`
	SuccessRate          float64  `json:"success_rate"`
	CompleteRate         float64  `json:"complete_rate"`
	MedianTTFBMS         int64    `json:"median_ttfb_ms,omitempty"`
	MedianDurationMS     int64    `json:"median_duration_ms,omitempty"`
	MedianThroughput     int64    `json:"median_throughput_bps,omitempty"`
	VerifiedCount        int      `json:"verified_count"`
	ObservationCount     int      `json:"observation_count,omitempty"`
	WorkingObservations  int      `json:"working_observations,omitempty"`
	UnstableObservations int      `json:"unstable_observations,omitempty"`
	FailureObservations  int      `json:"failure_observations,omitempty"`
	EnvironmentChanges   int      `json:"environment_changes,omitempty"`
	SuccessStreak        int      `json:"success_streak"`
	FailureStreak        int      `json:"failure_streak"`
	LastVerified         string   `json:"last_verified"`
	LastWorking          string   `json:"last_working,omitempty"`
	InfrastructureOK     bool     `json:"infrastructure_ok"`
	CleanupProven        bool     `json:"cleanup_proven"`
}

type v2TargetMemoryDocument struct {
	Version int                   `json:"version"`
	Entries []v2TargetMemoryEntry `json:"entries"`
}

const (
	v2MemoryPolicyVersion       = 1
	v2MemoryWorkingMaxAge       = 14 * 24 * time.Hour
	v2MemoryUnstableMaxAge      = 6 * time.Hour
	v2MemoryStrongMaxAge        = 72 * time.Hour
	v2MemoryTrustedMaxAge       = 24 * time.Hour
	v2MemoryFutureSkew          = 5 * time.Minute
	v2MemoryUnstableMinVerified = 2
	v2MemoryStrongMinVerified   = 2
	v2MemoryStrongMinStreak     = 2
	v2MemoryTrustedMinVerified  = 3
	v2MemoryTrustedMinStreak    = 3
)

const (
	v2MemoryConfidenceTrusted   = "TRUSTED"
	v2MemoryConfidenceStrong    = "STRONG"
	v2MemoryConfidenceFresh     = "FRESH"
	v2MemoryConfidenceProbation = "PROBATION"
	v2MemoryConfidenceStale     = "STALE"
	v2MemoryConfidenceBlocked   = "BLOCKED"
)

const v2MemoryUnstableMinSuccessRate = 0.5

type v2TargetMemoryPolicy struct {
	Version                 int     `json:"version"`
	WorkingMaxAgeSeconds    int64   `json:"working_max_age_seconds"`
	UnstableMaxAgeSeconds   int64   `json:"unstable_max_age_seconds"`
	StrongMaxAgeSeconds     int64   `json:"strong_max_age_seconds"`
	TrustedMaxAgeSeconds    int64   `json:"trusted_max_age_seconds"`
	FutureSkewSeconds       int64   `json:"future_skew_seconds"`
	UnstableMinVerified     int     `json:"unstable_min_verified"`
	UnstableMinSuccessRate  float64 `json:"unstable_min_success_rate"`
	StrongMinVerified       int     `json:"strong_min_verified"`
	StrongMinSuccessStreak  int     `json:"strong_min_success_streak"`
	TrustedMinVerified      int     `json:"trusted_min_verified"`
	TrustedMinSuccessStreak int     `json:"trusted_min_success_streak"`
}

type v2TargetMemoryReuseDecision struct {
	ReuseEligible bool
	Confidence    string
	AgeSeconds    int64
	Reason        string
}

func v2TargetMemoryPolicySnapshot() v2TargetMemoryPolicy {
	return v2TargetMemoryPolicy{
		Version:                 v2MemoryPolicyVersion,
		WorkingMaxAgeSeconds:    int64(v2MemoryWorkingMaxAge / time.Second),
		UnstableMaxAgeSeconds:   int64(v2MemoryUnstableMaxAge / time.Second),
		StrongMaxAgeSeconds:     int64(v2MemoryStrongMaxAge / time.Second),
		TrustedMaxAgeSeconds:    int64(v2MemoryTrustedMaxAge / time.Second),
		FutureSkewSeconds:       int64(v2MemoryFutureSkew / time.Second),
		UnstableMinVerified:     v2MemoryUnstableMinVerified,
		UnstableMinSuccessRate:  v2MemoryUnstableMinSuccessRate,
		StrongMinVerified:       v2MemoryStrongMinVerified,
		StrongMinSuccessStreak:  v2MemoryStrongMinStreak,
		TrustedMinVerified:      v2MemoryTrustedMinVerified,
		TrustedMinSuccessStreak: v2MemoryTrustedMinStreak,
	}
}

func v2TargetMemoryReuseDecisionForEntry(entry v2TargetMemoryEntry, now time.Time) v2TargetMemoryReuseDecision {
	decision := v2TargetMemoryReuseDecision{
		ReuseEligible: false,
		Confidence:    v2MemoryConfidenceBlocked,
		AgeSeconds:    -1,
		Reason:        "result class is not reusable",
	}
	if !entry.InfrastructureOK || !entry.CleanupProven {
		decision.Reason = "infrastructure or cleanup proof is not valid"
		return decision
	}
	if entry.FailureStreak > 0 {
		decision.Reason = "failure streak is non-zero"
		return decision
	}
	verifiedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(entry.LastVerified))
	if err != nil {
		decision.Reason = "last_verified is missing or invalid"
		return decision
	}
	now = now.UTC()
	verifiedAt = verifiedAt.UTC()
	if verifiedAt.After(now.Add(v2MemoryFutureSkew)) {
		decision.Reason = "last_verified is too far in the future"
		return decision
	}
	age := now.Sub(verifiedAt)
	if age < 0 {
		age = 0
	}
	decision.AgeSeconds = int64(age / time.Second)

	switch entry.ResultClass {
	case "WORKING":
		if entry.VerifiedCount < 1 || entry.SuccessStreak < 1 {
			decision.Reason = "working evidence has no successful verification streak"
			return decision
		}
		if age > v2MemoryWorkingMaxAge {
			decision.Confidence = v2MemoryConfidenceStale
			decision.Reason = "working evidence is older than reuse TTL"
			return decision
		}
		decision.ReuseEligible = true
		decision.Confidence = v2MemoryConfidenceFresh
		decision.Reason = "working evidence is fresh"
		if entry.VerifiedCount >= v2MemoryStrongMinVerified && entry.SuccessStreak >= v2MemoryStrongMinStreak && age <= v2MemoryStrongMaxAge {
			decision.Confidence = v2MemoryConfidenceStrong
			decision.Reason = "working evidence is repeated"
		}
		if entry.VerifiedCount >= v2MemoryTrustedMinVerified && entry.SuccessStreak >= v2MemoryTrustedMinStreak && age <= v2MemoryTrustedMaxAge {
			decision.Confidence = v2MemoryConfidenceTrusted
			decision.Reason = "working evidence is repeatedly verified"
		}
		return decision
	case "UNSTABLE":
		decision.Confidence = v2MemoryConfidenceProbation
		if age > v2MemoryUnstableMaxAge {
			decision.Confidence = v2MemoryConfidenceStale
			decision.Reason = "unstable evidence is older than probation TTL"
			return decision
		}
		if entry.VerifiedCount < v2MemoryUnstableMinVerified {
			decision.Reason = "unstable evidence needs at least two independent verifications"
			return decision
		}
		if entry.SuccessRate < v2MemoryUnstableMinSuccessRate {
			decision.Reason = "unstable evidence success rate is below probation threshold"
			return decision
		}
		decision.ReuseEligible = true
		decision.Reason = "recent repeated unstable evidence is probationary"
		return decision
	default:
		return decision
	}
}

func v2MemoryConfidenceRank(value string) int {
	switch value {
	case v2MemoryConfidenceTrusted:
		return 4
	case v2MemoryConfidenceStrong:
		return 3
	case v2MemoryConfidenceFresh:
		return 2
	case v2MemoryConfidenceProbation:
		return 1
	default:
		return 0
	}
}

func v2TargetMemoryEntryView(entry v2TargetMemoryEntry, now time.Time) map[string]any {
	view := map[string]any{}
	if data, err := json.Marshal(entry); err == nil {
		_ = json.Unmarshal(data, &view)
	}
	decision := v2TargetMemoryReuseDecisionForEntry(entry, now)
	view["reuse_eligible"] = decision.ReuseEligible
	view["confidence"] = decision.Confidence
	view["age_seconds"] = decision.AgeSeconds
	view["reuse_reason"] = decision.Reason
	return view
}

func registerTargetMemoryV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/memory", getOnly(handleV2TargetMemory))
	mux.HandleFunc("/v1/v2/memory/clear", mutationOnly(handleV2TargetMemoryClear))
}

func v2MemoryEnvironmentFingerprintForProtocol(configSHA, protocol string) string {
	protocol = strings.ToLower(strings.TrimSpace(protocol))
	if protocol == "" {
		protocol = benchTransportHTTPS
	}
	h := sha256.New()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(configSHA))))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(runtime.GOARCH))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(protocol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("ipv4"))
	return hex.EncodeToString(h.Sum(nil))
}

func v2MemoryEnvironmentFingerprint(configSHA string) string {
	return v2MemoryEnvironmentFingerprintForProtocol(configSHA, benchTransportHTTPS)
}

func ensureV2TargetMemoryRoot() error {
	if err := os.MkdirAll(v2TargetMemoryRoot, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(v2TargetMemoryRoot)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("target memory root is not a real directory")
	}
	return nil
}

func readV2TargetMemory() (v2TargetMemoryDocument, error) {
	doc := v2TargetMemoryDocument{Version: v2TargetMemoryVersion, Entries: []v2TargetMemoryEntry{}}
	data, err := os.ReadFile(v2TargetMemoryPath)
	if errors.Is(err, os.ErrNotExist) {
		return doc, nil
	}
	if err != nil {
		return doc, err
	}
	if len(data) > 2<<20 {
		return doc, errors.New("target memory exceeds safety limit")
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, errors.New("decode target memory: " + err.Error())
	}
	if doc.Version != v2TargetMemoryVersion {
		return doc, errors.New("unsupported target memory version")
	}
	if len(doc.Entries) > v2TargetMemoryMax {
		return doc, errors.New("target memory exceeds entry limit")
	}
	return doc, nil
}

func writeV2TargetMemory(doc v2TargetMemoryDocument) error {
	if err := ensureV2TargetMemoryRoot(); err != nil {
		return err
	}
	if len(doc.Entries) > v2TargetMemoryMax {
		return errors.New("target memory entry limit exceeded")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if old, readErr := os.ReadFile(v2TargetMemoryPath); readErr == nil && len(old) > 0 {
		_, _ = v2TargetMemoryBackup("target-memory", "target-memory", v2TargetMemoryPath, old, 0600)
	}
	return safety.WriteFileAtomic(v2TargetMemoryPath, data, 0600)
}

func v2MemoryKey(target, protocol, fingerprint string) string {
	return strings.ToLower(strings.TrimSpace(target)) + "\x00" +
		strings.ToLower(strings.TrimSpace(protocol)) + "\x00" +
		strings.ToLower(strings.TrimSpace(fingerprint))
}

func v2HydrateLegacyObservationCounters(entry *v2TargetMemoryEntry) {
	if entry == nil || entry.ObservationCount > 0 || entry.VerifiedCount <= 0 {
		return
	}
	entry.ObservationCount = entry.VerifiedCount
	switch entry.ResultClass {
	case "WORKING":
		entry.WorkingObservations = entry.VerifiedCount
	case "UNSTABLE":
		entry.UnstableObservations = entry.VerifiedCount
	case "FAILED", "PARTIAL":
		entry.FailureObservations = entry.VerifiedCount
	}
}

func v2ResetMemoryEvidenceForEnvironment(entry *v2TargetMemoryEntry) {
	if entry == nil {
		return
	}
	entry.SuccessRate = 0
	entry.CompleteRate = 0
	entry.MedianTTFBMS = 0
	entry.MedianDurationMS = 0
	entry.MedianThroughput = 0
	entry.VerifiedCount = 0
	entry.ObservationCount = 0
	entry.WorkingObservations = 0
	entry.UnstableObservations = 0
	entry.FailureObservations = 0
	entry.SuccessStreak = 0
	entry.FailureStreak = 0
	entry.LastVerified = ""
	entry.LastWorking = ""
}

func v2MergeTargetMemoryObservation(entry v2TargetMemoryEntry, candidate v2CandidateResult, env, configSHA, now string) v2TargetMemoryEntry {
	env = strings.TrimSpace(env)
	if entry.Environment != "" && !strings.EqualFold(entry.Environment, env) {
		entry.EnvironmentChanges++
		v2ResetMemoryEvidenceForEnvironment(&entry)
	} else {
		v2HydrateLegacyObservationCounters(&entry)
	}

	previous := entry.VerifiedCount
	entry.CandidateID = candidate.CandidateID
	entry.CandidateName = candidate.CandidateName
	entry.CandidateSource = candidate.CandidateSource
	entry.Environment = env
	entry.ConfigSHA256 = strings.ToLower(strings.TrimSpace(configSHA))
	entry.Args = append([]string{}, v2PortableCandidateArgs(candidate.Args)...)
	entry.ResultClass = candidate.ResultClass
	entry.SuccessRate = (entry.SuccessRate*float64(previous) + candidate.SuccessRate) / float64(previous+1)
	entry.CompleteRate = (entry.CompleteRate*float64(previous) + candidate.CompleteRate) / float64(previous+1)
	entry.MedianTTFBMS = candidate.MedianTTFBMS
	entry.MedianDurationMS = candidate.MedianDurationMS
	entry.MedianThroughput = candidate.MedianThroughput
	entry.VerifiedCount = previous + 1
	entry.ObservationCount++
	entry.LastVerified = now
	entry.InfrastructureOK = candidate.InfrastructureOK
	entry.CleanupProven = candidate.CleanupProven

	switch candidate.ResultClass {
	case "WORKING":
		entry.WorkingObservations++
		entry.SuccessStreak++
		entry.FailureStreak = 0
		entry.LastWorking = now
	case "UNSTABLE":
		entry.UnstableObservations++
		entry.SuccessStreak = 0
		entry.FailureStreak = 0
	case "FAILED", "PARTIAL":
		entry.FailureObservations++
		entry.FailureStreak++
		entry.SuccessStreak = 0
	default:
		entry.SuccessStreak = 0
		entry.FailureStreak = 0
	}
	return entry
}

func v2RecordSelectorEvidence(target, configSHA string, candidates []v2CandidateResult) error {
	doc, err := readV2TargetMemory()
	if err != nil {
		return err
	}
	target, err = v2NormalizeTarget(target)
	if err != nil {
		return err
	}
	now := v2TargetMemoryNow().UTC().Format(time.RFC3339)
	env := v2MemoryEnvironmentFingerprint(configSHA)

	index := map[string]int{}
	for i := range doc.Entries {
		index[v2MemoryKey(doc.Entries[i].Target, doc.Entries[i].Protocol, doc.Entries[i].Fingerprint)] = i
	}

	changed := false
	for _, candidate := range candidates {
		if !candidate.InfrastructureOK || !candidate.CleanupProven || len(candidate.Args) == 0 {
			continue
		}
		args := v2PortableCandidateArgs(candidate.Args)
		if len(args) == 0 {
			continue
		}
		fp := v2CandidateTechniqueFingerprint(args)
		if fp == "" {
			continue
		}
		key := v2MemoryKey(target, "https", fp)
		entry := v2TargetMemoryEntry{}
		if i, ok := index[key]; ok {
			entry = doc.Entries[i]
		}
		entry.Target = target
		entry.Protocol = "https"
		entry.IPFamily = "ipv4"
		entry.Fingerprint = fp
		entry = v2MergeTargetMemoryObservation(entry, candidate, env, configSHA, now)

		if i, ok := index[key]; ok {
			doc.Entries[i] = entry
		} else {
			index[key] = len(doc.Entries)
			doc.Entries = append(doc.Entries, entry)
		}
		changed = true
	}
	if !changed {
		return nil
	}

	sort.SliceStable(doc.Entries, func(i, j int) bool {
		return doc.Entries[i].LastVerified > doc.Entries[j].LastVerified
	})
	if len(doc.Entries) > v2TargetMemoryMax {
		doc.Entries = doc.Entries[:v2TargetMemoryMax]
	}
	return writeV2TargetMemory(doc)
}

func v2TargetMemoryCandidatesForTransport(target, configSHA, protocol string, limit int) ([]v2CandidatePoolItem, error) {
	doc, err := readV2TargetMemory()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 8
	}
	transport, err := normalizeBenchTransport(protocol)
	if err != nil {
		return nil, err
	}
	env := v2MemoryEnvironmentFingerprintForProtocol(configSHA, transport.ID)
	target, err = v2NormalizeTarget(target)
	if err != nil {
		return nil, err
	}
	now := v2TargetMemoryNow().UTC()

	type rankedMemoryEntry struct {
		Entry    v2TargetMemoryEntry
		Decision v2TargetMemoryReuseDecision
	}
	entries := make([]rankedMemoryEntry, 0)
	for _, entry := range doc.Entries {
		if entry.Target != target || entry.Protocol != transport.ID || entry.IPFamily != "ipv4" ||
			entry.Environment != env {
			continue
		}
		decision := v2TargetMemoryReuseDecisionForEntry(entry, now)
		if !decision.ReuseEligible {
			continue
		}
		entries = append(entries, rankedMemoryEntry{Entry: entry, Decision: decision})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		ri := v2MemoryConfidenceRank(entries[i].Decision.Confidence)
		rj := v2MemoryConfidenceRank(entries[j].Decision.Confidence)
		if ri != rj {
			return ri > rj
		}
		if entries[i].Entry.SuccessStreak != entries[j].Entry.SuccessStreak {
			return entries[i].Entry.SuccessStreak > entries[j].Entry.SuccessStreak
		}
		if entries[i].Entry.VerifiedCount != entries[j].Entry.VerifiedCount {
			return entries[i].Entry.VerifiedCount > entries[j].Entry.VerifiedCount
		}
		if entries[i].Entry.SuccessRate != entries[j].Entry.SuccessRate {
			return entries[i].Entry.SuccessRate > entries[j].Entry.SuccessRate
		}
		if entries[i].Entry.CompleteRate != entries[j].Entry.CompleteRate {
			return entries[i].Entry.CompleteRate > entries[j].Entry.CompleteRate
		}
		if entries[i].Decision.AgeSeconds != entries[j].Decision.AgeSeconds {
			return entries[i].Decision.AgeSeconds < entries[j].Decision.AgeSeconds
		}
		return entries[i].Entry.Fingerprint < entries[j].Entry.Fingerprint
	})

	out := make([]v2CandidatePoolItem, 0, len(entries))
	for _, ranked := range entries {
		entry := ranked.Entry
		decision := ranked.Decision
		id := "memory-"
		if len(entry.Fingerprint) >= 16 {
			id += entry.Fingerprint[:16]
		} else {
			id += entry.Fingerprint
		}
		name := strings.TrimSpace(entry.CandidateName)
		if name == "" {
			name = "Verified memory candidate"
		}
		out = append(out, v2CandidatePoolItem{
			ID: id, Name: name, Source: "memory", Family: "verified",
			Protocol: entry.Protocol, Args: append([]string{}, entry.Args...),
			Fingerprint: entry.Fingerprint, MemoryClass: entry.ResultClass,
			MemoryConfidence: decision.Confidence, MemoryAgeSeconds: decision.AgeSeconds,
			MemoryVerifiedCount: entry.VerifiedCount, MemorySuccessStreak: entry.SuccessStreak,
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
func v2TargetMemoryCandidates(target, configSHA string, limit int) ([]v2CandidatePoolItem, error) {
	return v2TargetMemoryCandidatesForTransport(target, configSHA, benchTransportHTTPS, limit)
}
func handleV2TargetMemory(w http.ResponseWriter, r *http.Request) {
	doc, err := readV2TargetMemory()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if target != "" {
		normalized, normalizeErr := v2NormalizeTarget(target)
		if normalizeErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": normalizeErr.Error()})
			return
		}
		filtered := make([]v2TargetMemoryEntry, 0)
		for _, entry := range doc.Entries {
			if entry.Target == normalized {
				filtered = append(filtered, entry)
			}
		}
		doc.Entries = filtered
	}
	now := v2TargetMemoryNow().UTC()
	views := make([]map[string]any, 0, len(doc.Entries))
	for _, entry := range doc.Entries {
		views = append(views, v2TargetMemoryEntryView(entry, now))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": doc.Version, "feature_version": v2TargetMemoryFeatureVersion,
		"entries": views, "count": len(views), "policy": v2TargetMemoryPolicySnapshot(),
	})
}
func handleV2TargetMemoryClear(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target  string `json:"target,omitempty"`
		Confirm string `json:"confirm"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid memory clear request"})
		return
	}
	if req.Confirm != v2TargetMemoryConfirm {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "confirm must equal " + v2TargetMemoryConfirm})
		return
	}
	doc, err := readV2TargetMemory()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Target) == "" {
		doc.Entries = []v2TargetMemoryEntry{}
	} else {
		target, normalizeErr := v2NormalizeTarget(req.Target)
		if normalizeErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": normalizeErr.Error()})
			return
		}
		out := make([]v2TargetMemoryEntry, 0, len(doc.Entries))
		for _, entry := range doc.Entries {
			if entry.Target != target {
				out = append(out, entry)
			}
		}
		doc.Entries = out
	}
	if err := writeV2TargetMemory(doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "count": len(doc.Entries)})
}
