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
	v2TargetMemoryVersion = 1
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
	Target           string   `json:"target"`
	Protocol         string   `json:"protocol"`
	IPFamily         string   `json:"ip_family"`
	CandidateID      string   `json:"candidate_id"`
	CandidateName    string   `json:"candidate_name"`
	CandidateSource  string   `json:"candidate_source"`
	Fingerprint      string   `json:"fingerprint"`
	Environment      string   `json:"environment_fingerprint"`
	ConfigSHA256     string   `json:"config_sha256"`
	Args             []string `json:"args"`
	ResultClass      string   `json:"result_class"`
	SuccessRate      float64  `json:"success_rate"`
	CompleteRate     float64  `json:"complete_rate"`
	MedianTTFBMS     int64    `json:"median_ttfb_ms,omitempty"`
	MedianDurationMS int64    `json:"median_duration_ms,omitempty"`
	MedianThroughput int64    `json:"median_throughput_bps,omitempty"`
	VerifiedCount    int      `json:"verified_count"`
	SuccessStreak    int      `json:"success_streak"`
	FailureStreak    int      `json:"failure_streak"`
	LastVerified     string   `json:"last_verified"`
	LastWorking      string   `json:"last_working,omitempty"`
	InfrastructureOK bool     `json:"infrastructure_ok"`
	CleanupProven    bool     `json:"cleanup_proven"`
}

type v2TargetMemoryDocument struct {
	Version int                   `json:"version"`
	Entries []v2TargetMemoryEntry `json:"entries"`
}

func registerTargetMemoryV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/v2/memory", getOnly(handleV2TargetMemory))
	mux.HandleFunc("/v1/v2/memory/clear", mutationOnly(handleV2TargetMemoryClear))
}

func v2MemoryEnvironmentFingerprint(configSHA string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(configSHA))))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(runtime.GOARCH))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("https"))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("ipv4"))
	return hex.EncodeToString(h.Sum(nil))
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
		entry.CandidateID = candidate.CandidateID
		entry.CandidateName = candidate.CandidateName
		entry.CandidateSource = candidate.CandidateSource
		entry.Fingerprint = fp
		entry.Environment = env
		entry.ConfigSHA256 = strings.ToLower(strings.TrimSpace(configSHA))
		entry.Args = append([]string{}, args...)
		entry.ResultClass = candidate.ResultClass
		entry.SuccessRate = candidate.SuccessRate
		entry.CompleteRate = candidate.CompleteRate
		entry.MedianTTFBMS = candidate.MedianTTFBMS
		entry.MedianDurationMS = candidate.MedianDurationMS
		entry.MedianThroughput = candidate.MedianThroughput
		entry.VerifiedCount++
		entry.LastVerified = now
		entry.InfrastructureOK = candidate.InfrastructureOK
		entry.CleanupProven = candidate.CleanupProven

		switch candidate.ResultClass {
		case "WORKING":
			entry.SuccessStreak++
			entry.FailureStreak = 0
			entry.LastWorking = now
		case "FAILED", "PARTIAL":
			entry.FailureStreak++
			entry.SuccessStreak = 0
		default:
			entry.SuccessStreak = 0
			entry.FailureStreak = 0
		}

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

func v2TargetMemoryCandidates(target, configSHA string, limit int) ([]v2CandidatePoolItem, error) {
	doc, err := readV2TargetMemory()
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 8
	}
	env := v2MemoryEnvironmentFingerprint(configSHA)
	target, err = v2NormalizeTarget(target)
	if err != nil {
		return nil, err
	}

	entries := make([]v2TargetMemoryEntry, 0)
	for _, entry := range doc.Entries {
		if entry.Target != target || entry.Protocol != "https" || entry.IPFamily != "ipv4" ||
			entry.Environment != env || !entry.InfrastructureOK || !entry.CleanupProven {
			continue
		}
		if entry.ResultClass != "WORKING" && entry.ResultClass != "UNSTABLE" {
			continue
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ResultClass != entries[j].ResultClass {
			return entries[i].ResultClass == "WORKING"
		}
		if entries[i].SuccessRate != entries[j].SuccessRate {
			return entries[i].SuccessRate > entries[j].SuccessRate
		}
		if entries[i].CompleteRate != entries[j].CompleteRate {
			return entries[i].CompleteRate > entries[j].CompleteRate
		}
		return entries[i].LastVerified > entries[j].LastVerified
	})

	out := make([]v2CandidatePoolItem, 0, len(entries))
	for _, entry := range entries {
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
		})
		if len(out) >= limit {
			break
		}
	}
	return out, nil
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
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "version": doc.Version, "entries": doc.Entries, "count": len(doc.Entries),
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
