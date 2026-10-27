package dnsrouter

// DPR2-G accepts a bounded, non-secret observation summary collected by a
// read-only shell script. It is NOT a hardware-acceptance certificate and cannot
// initiate DNS configuration changes or any other router mutation.

import (
	"bufio"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const maxHardwareEvidenceBytes = 16 << 10

type HardwareAssessment struct {
	SchemaVersion     int      `json:"schema_version"`
	Mode              string   `json:"mode"`
	Classification    string   `json:"classification"`
	Observed          []string `json:"observed"`
	Blockers          []string `json:"blockers"`
	NextTests         []string `json:"next_tests"`
	ActivationAllowed bool     `json:"activation_allowed"`
	HardwareAccepted  bool     `json:"hardware_accepted"`
	ChangesApplied    bool     `json:"changes_applied"`
}

var hardwareEvidenceKeys = map[string]bool{
	"SCHEMA": true, "MODE": true, "ARCH": true, "ENTWARE_OPT": true,
	"PROC_MEMINFO": true, "PORT53_ENTRIES": true, "NDMC_VERSION": true,
	"NDMC_DNS_SHOW": true, "DNS_OVERRIDE": true,
	"PORT53_OWNER": true, "LOCAL_NAMES_BASELINE": true,
	"SHADOW_UDP_TCP": true, "RECURSION_TEST": true,
	"ROLLBACK_TEST": true,
}

func ParseHardwareReadOnlyEvidence(text string) (HardwareAssessment, error) {
	if len(text) == 0 || len(text) > maxHardwareEvidenceBytes {
		return HardwareAssessment{}, errors.New("hardware evidence must be nonempty and <= 16 KiB")
	}
	const begin, end = "RF_DPR2G_EVIDENCE_BEGIN", "RF_DPR2G_EVIDENCE_END"
	var inside, finished bool
	observations := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == begin {
			if inside || finished {
				return HardwareAssessment{}, errors.New("duplicate hardware evidence begin marker")
			}
			inside = true
			continue
		}
		if line == end {
			if !inside || finished {
				return HardwareAssessment{}, errors.New("unexpected hardware evidence end marker")
			}
			finished = true
			inside = false
			continue
		}
		if !inside {
			if line != "" {
				return HardwareAssessment{}, errors.New("unexpected text outside evidence markers")
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || !hardwareEvidenceKeys[parts[0]] {
			return HardwareAssessment{}, errors.New("unknown or malformed hardware evidence field")
		}
		if _, exists := observations[parts[0]]; exists {
			return HardwareAssessment{}, fmt.Errorf("duplicate evidence field %s", parts[0])
		}
		if len(parts[1]) > 64 || strings.ContainsAny(parts[1], " \t\r\n\x00") {
			return HardwareAssessment{}, fmt.Errorf("invalid value for %s", parts[0])
		}
		observations[parts[0]] = parts[1]
	}
	if err := scanner.Err(); err != nil {
		return HardwareAssessment{}, err
	}
	if !finished || inside || observations["SCHEMA"] != "1" || observations["MODE"] != "READ_ONLY" {
		return HardwareAssessment{}, errors.New("incomplete or untrusted read-only hardware evidence")
	}
	port53, err := strconv.Atoi(observations["PORT53_ENTRIES"])
	if err != nil || port53 < 0 || port53 > 256 {
		return HardwareAssessment{}, errors.New("invalid port 53 observation count")
	}
	allowed := map[string]bool{"OK": true, "UNKNOWN": true, "UNAVAILABLE": true, "NOT_TESTED": true}
	for _, key := range []string{"NDMC_VERSION", "NDMC_DNS_SHOW", "DNS_OVERRIDE", "PORT53_OWNER", "LOCAL_NAMES_BASELINE", "SHADOW_UDP_TCP", "RECURSION_TEST", "ROLLBACK_TEST"} {
		if !allowed[observations[key]] {
			return HardwareAssessment{}, fmt.Errorf("invalid status for %s", key)
		}
	}
	if observations["ENTWARE_OPT"] != "AVAILABLE" && observations["ENTWARE_OPT"] != "UNAVAILABLE" {
		return HardwareAssessment{}, errors.New("invalid Entware status")
	}
	if observations["PROC_MEMINFO"] != "AVAILABLE" && observations["PROC_MEMINFO"] != "UNAVAILABLE" {
		return HardwareAssessment{}, errors.New("invalid proc memory status")
	}
	result := HardwareAssessment{
		SchemaVersion: 1, Mode: "offline-read-only-assessment", Classification: "INSUFFICIENT_EVIDENCE",
		Observed:          []string{fmt.Sprintf("port 53 socket-table entries: %d (ownership not proven)", port53)},
		Blockers:          []string{"DNS Proxy ownership and configuration are not independently proven", "global upstream selection semantics for this firmware are not verified"},
		NextTests:         []string{"separately authorized shadow UDP/TCP hardware test", "snapshot exact Keenetic DNS settings before an activation experiment", "baseline local names and A/AAAA over both transports", "loop detection with selected upstreams", "unattended rollback after simulated failure"},
		ActivationAllowed: false, HardwareAccepted: false, ChangesApplied: false,
	}
	if observations["NDMC_VERSION"] == "OK" {
		result.Observed = append(result.Observed, "read-only ndmc version command succeeded")
	}
	if observations["NDMC_DNS_SHOW"] == "OK" {
		result.Observed = append(result.Observed, "read-only ndmc name-server inventory command succeeded")
	}
	if observations["ENTWARE_OPT"] == "AVAILABLE" {
		result.Observed = append(result.Observed, "Entware /opt path exists")
	} else {
		result.Blockers = append(result.Blockers, "Entware /opt unavailable")
	}
	for _, key := range []string{"DNS_OVERRIDE", "PORT53_OWNER", "LOCAL_NAMES_BASELINE", "SHADOW_UDP_TCP", "RECURSION_TEST", "ROLLBACK_TEST"} {
		if observations[key] != "OK" {
			result.Blockers = append(result.Blockers, strings.ToLower(key)+" is not verified")
		}
	}
	if observations["NDMC_VERSION"] == "OK" && observations["NDMC_DNS_SHOW"] == "OK" && observations["ENTWARE_OPT"] == "AVAILABLE" {
		result.Classification = "READ_ONLY_INVENTORY_AVAILABLE"
	}
	// This code never returns authorization: even a perfect *user-supplied*
	// text report cannot prove physical provenance or approve live mutations.
	return result, nil
}
