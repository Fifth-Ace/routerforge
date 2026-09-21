package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	v2TCP16ProbeConfirm      = "ROUTERFORGE_TCP16_PROBE_V2"
	v2TCP16ChunkSize         = 4000
	v2TCP16ChunkCount        = 10
	v2TCP16MinDetectKB       = 12
	v2TCP16MaxTargets        = 6
	v2TCP16MaxSNICandidates  = 10
	v2TCP16MaxDetectedScans  = 2
	v2TCP16ProbeTotalTimeout = 110 * time.Second
)

var (
	v2TCP16ConnectTimeout   = 4 * time.Second
	v2TCP16HandshakeTimeout = 4 * time.Second
	v2TCP16ReadTimeoutMin   = 1200 * time.Millisecond
	v2TCP16ReadTimeoutMax   = 5 * time.Second
	v2TCP16ChunkDelay       = 40 * time.Millisecond
	v2TCP16Pad              = strings.Repeat("R", v2TCP16ChunkSize)
)

type v2TCP16ProbeTarget struct {
	ID       string `json:"id"`
	ASN      int    `json:"asn"`
	Provider string `json:"provider"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	SNI      string `json:"sni,omitempty"`
}

type v2TCP16ProbeAttempt struct {
	Target      v2TCP16ProbeTarget `json:"target"`
	SNI         string             `json:"sni,omitempty"`
	Alive       bool               `json:"alive"`
	Completed   bool               `json:"completed"`
	Detected    bool               `json:"detected"`
	DiedAtKB    int                `json:"died_at_kb,omitempty"`
	RTTMS       int64              `json:"rtt_ms,omitempty"`
	Error       string             `json:"error,omitempty"`
	BytesPadded int                `json:"bytes_padded"`
}

type v2TCP16ProbeRun struct {
	Target          v2TCP16ProbeTarget  `json:"target"`
	Baseline        v2TCP16ProbeAttempt `json:"baseline"`
	SNITries        int                 `json:"sni_tries"`
	WorkingSNI      string              `json:"working_sni,omitempty"`
	SNIRevalidated  bool                `json:"sni_revalidated"`
	SNIScanEligible bool                `json:"sni_scan_eligible"`
}

type v2TCP16ProbeRequest struct {
	TargetIDs            []string `json:"target_ids,omitempty"`
	ScanSNI              *bool    `json:"scan_sni,omitempty"`
	SNICandidates        []string `json:"sni_candidates,omitempty"`
	ExpectedConfigSHA256 string   `json:"expected_config_sha256"`
	Confirm              string   `json:"confirm"`
}

type v2TCP16ProbeResponse struct {
	OK                 bool                     `json:"ok"`
	ConfigSHA256       string                   `json:"config_sha256"`
	ReadOnlyProduction bool                     `json:"read_only_production"`
	MemoryUpdated      bool                     `json:"memory_updated"`
	TargetCount        int                      `json:"target_count"`
	AliveCount         int                      `json:"alive_count"`
	DetectedCount      int                      `json:"detected_count"`
	WorkingSNICount    int                      `json:"working_sni_count"`
	Results            []v2TCP16ProbeRun        `json:"results"`
}

var v2TCP16ProbeTargets = []v2TCP16ProbeTarget{
	{ID: "HE-01", ASN: 24940, Provider: "Hetzner", IP: "91.98.156.82", Port: 443},
	{ID: "AK-01", ASN: 20940, Provider: "Akamai", IP: "23.222.76.4", Port: 443},
	{ID: "AWS-02", ASN: 16509, Provider: "AWS", IP: "3.165.188.250", Port: 443},
	{ID: "FST-03", ASN: 54113, Provider: "Fastly GitHub", IP: "185.199.108.133", Port: 443, SNI: "release-assets.githubusercontent.com"},
	{ID: "GCR-01", ASN: 199524, Provider: "Gcore", IP: "62.112.221.146", Port: 443},
	{ID: "OR-01", ASN: 31898, Provider: "Oracle Cloud", IP: "153.72.114.110", Port: 443},
}

var v2TCP16DefaultSNICandidates = []string{
	"hcaptcha.com",
	"vk.com",
	"2gis.com",
	"2gis.ru",
	"300.ya.ru",
	"ad.adriver.ru",
	"3475482542.mc.yandex.ru",
	"742231.ms.ok.ru",
	"a.wb.ru",
	"ad.mail.ru",
}

func v2TCP16DrainHEAD(br *bufio.Reader) error {
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		if line == "\r\n" || line == "\n" {
			return nil
		}
	}
}

func v2TCP16FinishFailure(res v2TCP16ProbeAttempt, chunkIndex, sentKB int, err error) v2TCP16ProbeAttempt {
	if err != nil {
		res.Error = err.Error()
	}
	res.DiedAtKB = sentKB
	if chunkIndex == 0 {
		return res
	}
	res.Alive = true
	if sentKB >= v2TCP16MinDetectKB {
		res.Detected = true
	}
	return res
}

func v2TCP16ProbeOne(ctx context.Context, target v2TCP16ProbeTarget, sni string) v2TCP16ProbeAttempt {
	res := v2TCP16ProbeAttempt{Target: target, SNI: sni}
	dialer := net.Dialer{Timeout: v2TCP16ConnectTimeout}
	start := time.Now()
	raw, err := dialer.DialContext(ctx, "tcp4", net.JoinHostPort(target.IP, strconv.Itoa(target.Port)))
	if err != nil {
		res.Error = "tcp: " + err.Error()
		return res
	}
	defer raw.Close()

	var conn net.Conn = raw
	if target.Port != 80 {
		_ = raw.SetDeadline(time.Now().Add(v2TCP16HandshakeTimeout))
		tc := tls.Client(raw, &tls.Config{
			ServerName:         sni,
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: true, // measurement intentionally probes SNI behavior, not site identity
		})
		if err := tc.HandshakeContext(ctx); err != nil {
			res.Error = "tls: " + err.Error()
			return res
		}
		conn = tc
	}
	_ = raw.SetDeadline(time.Time{})
	res.RTTMS = time.Since(start).Milliseconds()

	host := target.IP
	if strings.TrimSpace(sni) != "" {
		host = strings.TrimSpace(sni)
	}
	br := bufio.NewReader(conn)
	readTimeout := v2TCP16ReadTimeoutMax

	for i := 0; i < v2TCP16ChunkCount; i++ {
		var b strings.Builder
		b.WriteString("HEAD / HTTP/1.1\r\nHost: ")
		b.WriteString(host)
		b.WriteString("\r\nUser-Agent: RouterForge-TCP16/2\r\nConnection: keep-alive\r\n")
		if i > 0 {
			b.WriteString("X-Pad: ")
			b.WriteString(v2TCP16Pad)
			b.WriteString("\r\n")
			res.BytesPadded += v2TCP16ChunkSize
		}
		b.WriteString("\r\n")

		sentKB := i * v2TCP16ChunkSize / 1024
		_ = conn.SetDeadline(time.Now().Add(readTimeout))
		reqStart := time.Now()
		if _, err := conn.Write([]byte(b.String())); err != nil {
			return v2TCP16FinishFailure(res, i, sentKB, err)
		}
		if err := v2TCP16DrainHEAD(br); err != nil {
			return v2TCP16FinishFailure(res, i, sentKB, err)
		}
		if i == 0 {
			res.Alive = true
			readTimeout = time.Since(reqStart) * 3
			if readTimeout < v2TCP16ReadTimeoutMin {
				readTimeout = v2TCP16ReadTimeoutMin
			}
			if readTimeout > v2TCP16ReadTimeoutMax {
				readTimeout = v2TCP16ReadTimeoutMax
			}
		}
		if err := contextSleep(ctx, v2TCP16ChunkDelay); err != nil {
			res.Error = err.Error()
			return res
		}
	}
	res.Completed = true
	return res
}

func contextSleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func v2TCP16AttemptPassesSNI(res v2TCP16ProbeAttempt) bool {
	return res.Alive && res.Completed && !res.Detected && res.Error == ""
}

func v2TCP16SelectTargets(ids []string) ([]v2TCP16ProbeTarget, error) {
	if len(ids) == 0 {
		out := make([]v2TCP16ProbeTarget, 4)
		copy(out, v2TCP16ProbeTargets[:4])
		return out, nil
	}
	if len(ids) > v2TCP16MaxTargets {
		return nil, errors.New("tcp16 target_ids exceed limit")
	}
	index := map[string]v2TCP16ProbeTarget{}
	for _, item := range v2TCP16ProbeTargets {
		index[item.ID] = item
	}
	seen := map[string]bool{}
	out := []v2TCP16ProbeTarget{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" || seen[id] {
			continue
		}
		item, ok := index[id]
		if !ok {
			return nil, errors.New("unknown tcp16 target_id: " + id)
		}
		seen[id] = true
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil, errors.New("no tcp16 targets selected")
	}
	return out, nil
}

func v2TCP16NormalizeSNICandidates(values []string) ([]string, error) {
	if len(values) == 0 {
		return append([]string{}, v2TCP16DefaultSNICandidates...), nil
	}
	if len(values) > v2TCP16MaxSNICandidates {
		return nil, errors.New("tcp16 sni_candidates exceed limit")
	}
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range values {
		host, err := v2NormalizeTarget(raw)
		if err != nil {
			return nil, errors.New("invalid tcp16 SNI candidate: " + strings.TrimSpace(raw))
		}
		if seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	if len(out) == 0 {
		return nil, errors.New("no valid tcp16 SNI candidates")
	}
	return out, nil
}

func validateV2TCP16ProbeRequest(req v2TCP16ProbeRequest) error {
	if req.Confirm != v2TCP16ProbeConfirm {
		return errors.New("confirm must equal " + v2TCP16ProbeConfirm)
	}
	if !smartApplyHashPattern.MatchString(strings.TrimSpace(req.ExpectedConfigSHA256)) {
		return errors.New("expected_config_sha256 must be SHA256")
	}
	if len(req.TargetIDs) > v2TCP16MaxTargets {
		return errors.New("tcp16 target_ids exceed limit")
	}
	if len(req.SNICandidates) > v2TCP16MaxSNICandidates {
		return errors.New("tcp16 sni_candidates exceed limit")
	}
	return nil
}

func v2RecordTCP16ProbeResult(target v2TCP16ProbeTarget, baseline v2TCP16ProbeAttempt, workingSNI string) error {
	key, cidr, err := v2TCP16NetworkIdentity(target.IP)
	if err != nil {
		return err
	}
	doc, err := readV2TCP16Memory()
	if err != nil {
		return err
	}
	now := v2TCP16MemoryNow().UTC().Format(time.RFC3339)
	index := -1
	for i := range doc.Entries {
		if doc.Entries[i].NetworkKey == key {
			index = i
			break
		}
	}
	entry := v2TCP16NetworkEntry{
		NetworkKey:      key,
		CIDR:            cidr,
		DestinationIPv4: target.IP,
	}
	if index >= 0 {
		entry = doc.Entries[index]
	}
	entry.NetworkKey = key
	entry.CIDR = cidr
	entry.ASN = target.ASN
	entry.Provider = target.Provider
	entry.DestinationIPv4 = target.IP
	entry.LastTarget = target.ID
	entry.LastObserved = now
	entry.ObservationSource = "routerforge-tcp16-probe-v2"
	entry.CutoffSuspected = baseline.Detected
	if baseline.Detected {
		entry.SuspectedCount++
	} else if baseline.Alive && baseline.Completed {
		entry.ClearCount++
	}
	if workingSNI != "" {
		entry.WorkingSNI = workingSNI
		entry.WorkingSNISource = "routerforge-tcp16-probe-v2-revalidated"
		entry.LastWorkingSNI = now
	}
	entry.Confidence = v2TCP16Confidence(entry)

	if index >= 0 {
		doc.Entries[index] = entry
	} else {
		doc.Entries = append(doc.Entries, entry)
	}
	if len(doc.Entries) > v2TCP16MemoryMax {
		doc.Entries = doc.Entries[:v2TCP16MemoryMax]
	}
	return writeV2TCP16Memory(doc)
}

func handleV2TCP16Probe(w http.ResponseWriter, r *http.Request) {
	var req v2TCP16ProbeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid tcp16 probe request"})
		return
	}
	if err := validateV2TCP16ProbeRequest(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	status := readStatus()
	expectedSHA := strings.TrimSpace(req.ExpectedConfigSHA256)
	if !strings.EqualFold(status.ConfigSHA256, expectedSHA) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed before tcp16 probe",
			"current_sha256": status.ConfigSHA256,
		})
		return
	}
	targets, err := v2TCP16SelectTargets(req.TargetIDs)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	candidates, err := v2TCP16NormalizeSNICandidates(req.SNICandidates)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	scanSNI := true
	if req.ScanSNI != nil {
		scanSNI = *req.ScanSNI
	}

	ctx, cancel := context.WithTimeout(r.Context(), v2TCP16ProbeTotalTimeout)
	defer cancel()

	runs := make([]v2TCP16ProbeRun, 0, len(targets))
	aliveCount, detectedCount, workingCount, scannedNetworks := 0, 0, 0, 0

	for _, target := range targets {
		baselineSNI := strings.TrimSpace(target.SNI)
		baseline := v2TCP16ProbeOne(ctx, target, baselineSNI)
		run := v2TCP16ProbeRun{
			Target: target, Baseline: baseline,
			SNIScanEligible: baseline.Detected && target.Port == 443,
		}
		if baseline.Alive {
			aliveCount++
		}
		if baseline.Detected {
			detectedCount++
		}

		if scanSNI && run.SNIScanEligible && scannedNetworks < v2TCP16MaxDetectedScans {
			scannedNetworks++
			for _, candidate := range candidates {
				run.SNITries++
				first := v2TCP16ProbeOne(ctx, target, candidate)
				if !v2TCP16AttemptPassesSNI(first) {
					if ctx.Err() != nil {
						break
					}
					continue
				}
				second := v2TCP16ProbeOne(ctx, target, candidate)
				if v2TCP16AttemptPassesSNI(second) {
					run.WorkingSNI = candidate
					run.SNIRevalidated = true
					workingCount++
					break
				}
				if ctx.Err() != nil {
					break
				}
			}
		}
		runs = append(runs, run)
		if ctx.Err() != nil {
			break
		}
	}

	if current := readStatus().ConfigSHA256; !strings.EqualFold(current, expectedSHA) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":          "production config changed during tcp16 probe; evidence was not persisted",
			"current_sha256": current,
			"results":        runs,
		})
		return
	}

	for _, run := range runs {
		if err := v2RecordTCP16ProbeResult(run.Target, run.Baseline, run.WorkingSNI); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":   "persist tcp16 evidence: " + err.Error(),
				"results": runs,
			})
			return
		}
	}

	resp := v2TCP16ProbeResponse{
		OK:                 aliveCount > 0,
		ConfigSHA256:       expectedSHA,
		ReadOnlyProduction: true,
		MemoryUpdated:      true,
		TargetCount:        len(runs),
		AliveCount:         aliveCount,
		DetectedCount:      detectedCount,
		WorkingSNICount:    workingCount,
		Results:            runs,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

func v2TCP16ProbeSummary(run v2TCP16ProbeRun) string {
	state := "inconclusive"
	switch {
	case run.Baseline.Detected:
		state = "detected"
	case run.Baseline.Alive && run.Baseline.Completed:
		state = "clear"
	}
	if run.WorkingSNI != "" {
		state += ":sni=" + run.WorkingSNI
	}
	return fmt.Sprintf("%s:%s", run.Target.ID, state)
}
