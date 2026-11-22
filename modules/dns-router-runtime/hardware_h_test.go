package dnsrouter

import (
	"strings"
	"testing"
)

const hEvidence = `RF_DPR2H_EVIDENCE_BEGIN
SCHEMA=1
MODE=READ_ONLY
ARCH=aarch64
DSTEST_IP_MATCH=YES
OPT_DIR=YES
ENTWARE_OPKG=YES
MEM_AVAILABLE_KIB=128000
OPT_FREE_KIB=152000
PORT53_UDP_ENTRIES=2
PORT53_TCP_ENTRIES=1
SHADOW_55353_ENTRIES=0
NDMC_VERSION=OK
NDMC_DNS=OK
DNS_OVERRIDE=UNKNOWN
LOCAL_NAMES=NOT_TESTED
SHADOW_UDP_TCP=NOT_TESTED
LOOP_DETECTION=NOT_TESTED
ROLLBACK=NOT_TESTED
DNS_INTERCEPTION=NONE
NFQUEUE_300=NOT_TOUCHED
RF_DPR2H_EVIDENCE_END
`

func TestDPR2HValidReadOnly(t *testing.T) {
	r, err := ParseHardwareHReadiness(hEvidence)
	if err != nil {
		t.Fatal(err)
	}
	if !r.DeviceIdentified || r.Classification != "READ_ONLY_DSTEST_INVENTORY" {
		t.Fatalf("not classified: %+v", r)
	}
	if r.HardwareAccepted || r.ActivationAllowed || r.ChangesApplied || r.ShadowHardwarePass {
		t.Fatalf("unsafe positive result: %+v", r)
	}
}
func TestDPR2HRejectTruncationAndSpoof(t *testing.T) {
	for _, tc := range []string{
		strings.Replace(hEvidence, "RF_DPR2H_EVIDENCE_END", "", 1),
		strings.Replace(hEvidence, "DSTEST_IP_MATCH=YES", "DSTEST_IP_MATCH=YES\nDSTEST_IP_MATCH=YES", 1),
		strings.Replace(hEvidence, "LOCAL_NAMES=NOT_TESTED", "LOCAL_NAMES=OK", 1),
		strings.Replace(hEvidence, "SHADOW_UDP_TCP=NOT_TESTED", "SHADOW_UDP_TCP=OK", 1),
		strings.Replace(hEvidence, "NFQUEUE_300=NOT_TOUCHED", "NFQUEUE_300=MODIFIED", 1),
		strings.Replace(hEvidence, "MEM_AVAILABLE_KIB=128000", "MEM_AVAILABLE_KIB=-1", 1),
		strings.Replace(hEvidence, "ARCH=aarch64", "ARCH=aarch64\nCOMMAND=reload", 1),
	} {
		if _, err := ParseHardwareHReadiness(tc); err == nil {
			t.Fatal("forged or malformed evidence accepted")
		}
	}
}
func TestDPR2HFailClosedWrongHost(t *testing.T) {
	for _, repl := range []string{"NO", "UNKNOWN"} {
		raw := strings.Replace(hEvidence, "DSTEST_IP_MATCH=YES", "DSTEST_IP_MATCH="+repl, 1)
		r, err := ParseHardwareHReadiness(raw)
		if err != nil {
			t.Fatal(err)
		}
		if r.DeviceIdentified || r.Classification != "BLOCKED" {
			t.Fatal("wrong target accepted")
		}
	}
}
