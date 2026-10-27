package dnsrouter

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

// HardwareHReadiness is a read-only assessment of a collector's evidence.
// It is never an authorization to change Keenetic, launch a shadow resolver,
// or declare that hardware shadow acceptance has been performed.
type HardwareHReadiness struct {
	SchemaVersion      int      `json:"schema_version"`
	Mode               string   `json:"mode"`
	Classification     string   `json:"classification"`
	DeviceIdentified   bool     `json:"device_identified"`
	Observed           []string `json:"observed"`
	Blockers           []string `json:"blockers"`
	ShadowHardwarePass bool     `json:"shadow_hardware_pass"`
	HardwareAccepted   bool     `json:"hardware_accepted"`
	ActivationAllowed  bool     `json:"activation_allowed"`
	ChangesApplied     bool     `json:"changes_applied"`
}

var hardwareHFields = map[string]bool{
	"SCHEMA": true, "MODE": true, "ARCH": true, "DSTEST_IP_MATCH": true,
	"OPT_DIR": true, "ENTWARE_OPKG": true, "MEM_AVAILABLE_KIB": true,
	"OPT_FREE_KIB": true, "PORT53_UDP_ENTRIES": true, "PORT53_TCP_ENTRIES": true,
	"SHADOW_55353_ENTRIES": true, "NDMC_VERSION": true, "NDMC_DNS": true,
	"DNS_OVERRIDE": true, "LOCAL_NAMES": true, "SHADOW_UDP_TCP": true,
	"LOOP_DETECTION": true, "ROLLBACK": true, "DNS_INTERCEPTION": true,
	"NFQUEUE_300": true,
}

func ParseHardwareHReadiness(raw string) (HardwareHReadiness, error) {
	if len(raw) == 0 || len(raw) > 16384 {
		return HardwareHReadiness{}, fmt.Errorf("DPR2-H evidence must be between 1 and 16384 bytes")
	}
	const begin = "RF_DPR2H_EVIDENCE_BEGIN"
	const end = "RF_DPR2H_EVIDENCE_END"
	fields := make(map[string]string, len(hardwareHFields))
	open, closed := false, false
	s := bufio.NewScanner(strings.NewReader(raw))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		switch line {
		case begin:
			if open || closed {
				return HardwareHReadiness{}, fmt.Errorf("duplicate begin")
			}
			open = true
			continue
		case end:
			if !open || closed {
				return HardwareHReadiness{}, fmt.Errorf("unexpected end")
			}
			open, closed = false, true
			continue
		}
		if !open {
			if line != "" {
				return HardwareHReadiness{}, fmt.Errorf("text outside evidence frame")
			}
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 || !hardwareHFields[parts[0]] {
			return HardwareHReadiness{}, fmt.Errorf("unknown evidence field")
		}
		if _, present := fields[parts[0]]; present {
			return HardwareHReadiness{}, fmt.Errorf("duplicate field")
		}
		if parts[1] == "" || len(parts[1]) > 64 || strings.ContainsAny(parts[1], " \t\r\n\\/\x00") {
			return HardwareHReadiness{}, fmt.Errorf("invalid field value")
		}
		fields[parts[0]] = parts[1]
	}
	if err := s.Err(); err != nil {
		return HardwareHReadiness{}, err
	}
	if !closed || open || len(fields) != len(hardwareHFields) || fields["SCHEMA"] != "1" || fields["MODE"] != "READ_ONLY" {
		return HardwareHReadiness{}, fmt.Errorf("incomplete DPR2-H evidence")
	}
	for _, k := range []string{"PORT53_UDP_ENTRIES", "PORT53_TCP_ENTRIES", "SHADOW_55353_ENTRIES"} {
		value, err := strconv.Atoi(fields[k])
		if err != nil || value < 0 || value > 100000 {
			return HardwareHReadiness{}, fmt.Errorf("invalid %s", k)
		}
	}
	for _, k := range []string{"MEM_AVAILABLE_KIB", "OPT_FREE_KIB"} {
		if fields[k] == "UNKNOWN" {
			continue
		}
		value, err := strconv.ParseUint(fields[k], 10, 64)
		if err != nil || value > (1<<50) {
			return HardwareHReadiness{}, fmt.Errorf("invalid %s", k)
		}
	}
	for _, k := range []string{"OPT_DIR", "ENTWARE_OPKG"} {
		if fields[k] != "YES" && fields[k] != "NO" {
			return HardwareHReadiness{}, fmt.Errorf("invalid %s", k)
		}
	}
	for _, k := range []string{"NDMC_VERSION", "NDMC_DNS"} {
		if fields[k] != "OK" && fields[k] != "UNAVAILABLE" {
			return HardwareHReadiness{}, fmt.Errorf("invalid %s", k)
		}
	}
	if fields["DSTEST_IP_MATCH"] != "YES" && fields["DSTEST_IP_MATCH"] != "NO" && fields["DSTEST_IP_MATCH"] != "UNKNOWN" {
		return HardwareHReadiness{}, fmt.Errorf("invalid DSTest IP match")
	}
	if fields["DNS_OVERRIDE"] != "UNKNOWN" || fields["LOCAL_NAMES"] != "NOT_TESTED" || fields["SHADOW_UDP_TCP"] != "NOT_TESTED" || fields["LOOP_DETECTION"] != "NOT_TESTED" || fields["ROLLBACK"] != "NOT_TESTED" || fields["DNS_INTERCEPTION"] != "NONE" || fields["NFQUEUE_300"] != "NOT_TOUCHED" {
		return HardwareHReadiness{}, fmt.Errorf("DPR2-H collector scope violation")
	}
	device := (fields["ARCH"] == "aarch64" || fields["ARCH"] == "arm64") && fields["DSTEST_IP_MATCH"] == "YES"
	out := HardwareHReadiness{SchemaVersion: 1, Mode: "read-only-hardware-inventory", Classification: "BLOCKED", DeviceIdentified: device,
		Observed:           []string{"port 53 UDP entries: " + fields["PORT53_UDP_ENTRIES"], "port 53 TCP listeners: " + fields["PORT53_TCP_ENTRIES"], "high port entries: " + fields["SHADOW_55353_ENTRIES"], "available memory KiB: " + fields["MEM_AVAILABLE_KIB"], "free /opt KiB: " + fields["OPT_FREE_KIB"]},
		Blockers:           []string{"shadow UDP/TCP was not executed on DSTest", "local names, DNS loops and rollback have not been hardware-tested", "Keenetic DNS Proxy activation has no authorization"},
		ShadowHardwarePass: false, HardwareAccepted: false, ActivationAllowed: false, ChangesApplied: false,
	}
	if !device {
		out.Blockers = append(out.Blockers, "DSTest IPv4 identity or ARM64 architecture not proved")
	}
	if fields["NDMC_VERSION"] != "OK" || fields["NDMC_DNS"] != "OK" {
		out.Blockers = append(out.Blockers, "bounded NDMC show-only evidence unavailable")
	}
	if fields["ENTWARE_OPKG"] != "YES" {
		out.Blockers = append(out.Blockers, "Entware opkg unavailable")
	}
	if device && fields["NDMC_VERSION"] == "OK" && fields["NDMC_DNS"] == "OK" && fields["ENTWARE_OPKG"] == "YES" {
		out.Classification = "READ_ONLY_DSTEST_INVENTORY"
	}
	return out, nil
}
