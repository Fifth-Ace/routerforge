package dnsrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const dpr2GFixture = `RF_DPR2G_EVIDENCE_BEGIN
SCHEMA=1
MODE=READ_ONLY
ARCH=aarch64
ENTWARE_OPT=AVAILABLE
PROC_MEMINFO=AVAILABLE
PORT53_ENTRIES=2
NDMC_VERSION=OK
NDMC_DNS_SHOW=OK
DNS_OVERRIDE=UNKNOWN
PORT53_OWNER=UNKNOWN
LOCAL_NAMES_BASELINE=NOT_TESTED
SHADOW_UDP_TCP=NOT_TESTED
RECURSION_TEST=NOT_TESTED
ROLLBACK_TEST=NOT_TESTED
RF_DPR2G_EVIDENCE_END
`

func TestDPR2GReadOnlyEvidenceNeverActivates(t *testing.T) {
	a, err := ParseHardwareReadOnlyEvidence(dpr2GFixture)
	if err != nil {
		t.Fatal(err)
	}
	if a.ActivationAllowed || a.HardwareAccepted || a.ChangesApplied {
		t.Fatalf("unsafe approval %+v", a)
	}
	if a.Classification != "READ_ONLY_INVENTORY_AVAILABLE" || len(a.Blockers) < 6 {
		t.Fatalf("missing blockers: %+v", a)
	}
	raw := strings.ReplaceAll(dpr2GFixture, "=UNKNOWN", "=OK")
	raw = strings.ReplaceAll(raw, "=NOT_TESTED", "=OK")
	b, err := ParseHardwareReadOnlyEvidence(raw)
	if err != nil {
		t.Fatal(err)
	}
	if b.ActivationAllowed || b.HardwareAccepted || b.ChangesApplied {
		t.Fatal("forged all-OK observations activated router")
	}
}

func TestDPR2GRejectMalformedEvidence(t *testing.T) {
	examples := []string{
		"", strings.Repeat("A", 16385), "SCHEMA=1\nMODE=READ_ONLY\n",
		strings.Replace(dpr2GFixture, "MODE=READ_ONLY", "MODE=WRITABLE", 1),
		strings.Replace(dpr2GFixture, "SCHEMA=1", "SCHEMA=2", 1),
		strings.Replace(dpr2GFixture, "PORT53_ENTRIES=2", "PORT53_ENTRIES=bad", 1),
		strings.Replace(dpr2GFixture, "PORT53_ENTRIES=2", "PORT53_ENTRIES=-1", 1),
		strings.Replace(dpr2GFixture, "NDMC_DNS_SHOW=OK", "NDMC_DNS_SHOW=READY_FOR_ACTIVATION", 1),
		strings.Replace(dpr2GFixture, "NDMC_VERSION=OK", "NDMC_VERSION=OK\nNDMC_VERSION=OK", 1),
		strings.Replace(dpr2GFixture, "ARCH=aarch64", "ARCH=;rm -rf /", 1),
		dpr2GFixture + "UNSIGNED_TRAILING_COMMAND=1\n",
	}
	for i, example := range examples {
		if _, err := ParseHardwareReadOnlyEvidence(example); err == nil {
			t.Fatalf("invalid evidence %d accepted", i)
		}
	}
}

func TestDPR2GRehearsalAlwaysSimulationOnly(t *testing.T) {
	expectations := map[string]string{"none": "SIMULATED_COMMIT", "snapshot": "FAILED", "apply": "SIMULATED_ROLLBACK", "verify": "SIMULATED_ROLLBACK", "rollback": "AMBIGUOUS", "rollback-verify": "AMBIGUOUS", "invalid": "INVALID_SCENARIO"}
	for scenario, want := range expectations {
		r := RehearseKeeneticTransaction(scenario)
		if r.State != want || !r.SimulationOnly || r.ActivationAllowed || r.ChangesApplied || r.HardwareAccepted {
			t.Fatalf("%s: %+v", scenario, r)
		}
		if scenario == "verify" && (!r.BaselineRestored || len(r.Steps) < 4) {
			t.Fatalf("rollback evidence missing: %+v", r)
		}
		if scenario == "rollback" && r.BaselineRestored {
			t.Fatal("ambiguous rollback falsely proven")
		}
	}
}

func TestDPR2GOfflineAPIContract(t *testing.T) {
	api := NewAPI(NewConfigStore(t.TempDir()+"/config.json"), t.TempDir()).Handler()
	record := func(method, path, action, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if action != "" {
			r.Header.Set("X-RouterForge-Action", action)
		}
		if method == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		return w
	}
	payload, _ := json.Marshal(map[string]string{"evidence": dpr2GFixture})
	if w := record("POST", "/v1/keenetic/assess-hardware", "", string(payload)); w.Code != 403 {
		t.Fatal(w.Code)
	}
	w := record("POST", "/v1/keenetic/assess-hardware", "assess-hardware-readonly", string(payload))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"activation_allowed":false`) {
		t.Fatalf("API: %d %s", w.Code, w.Body.String())
	}
	if w := record("GET", "/v1/keenetic/assess-hardware", "", ""); w.Code != 405 {
		t.Fatal(w.Code)
	}
	for _, x := range []string{"verify", "none", "rollback", "rollback-verify"} {
		w := record("GET", "/v1/keenetic/rehearsal?failure="+x, "", "")
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"simulation_only":true`) {
			t.Fatalf("%s %d %s", x, w.Code, w.Body.String())
		}
	}
	if w := record("GET", "/v1/keenetic/rehearsal?failure=illegal", "", ""); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := record("POST", "/v1/keenetic/rehearsal", "", ""); w.Code != 405 {
		t.Fatal(w.Code)
	}
}
