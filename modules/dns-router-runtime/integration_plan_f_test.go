package dnsrouter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDPR2FIntegrationDesignAlwaysReadOnly(t *testing.T) {
	for _, n := range []int{0, 1, 7} {
		p := BuildKeeneticIntegrationPlan(KeeneticDiscovery{NDMCExecutableAvailable: true, OptAvailable: true, Port53Entries: n, ProcSocketTableAvailable: true})
		if p.ActivationAllowed || p.HardwareAccepted || p.ChangesApplied || p.Decision != "NO_ACTIVATION_BEFORE_EXPLICIT_HARDWARE_ACCEPTANCE" {
			t.Fatalf("unexpected approval for port53 entries %d: %+v", n, p)
		}
		if len(p.Candidates) != 4 || len(p.RollbackContract) < 4 || len(p.Unknowns) < 4 {
			t.Fatalf("missing evidence gates: %+v", p)
		}
		for _, c := range p.Candidates {
			if c.ID == "dns-override-or-redirection" && c.Classification != "NOT_APPROVED" {
				t.Fatalf("unsafe auto-selection: %+v", c)
			}
		}
	}
}

func TestDPR2FIntegrationPlanAPIIsReadOnly(t *testing.T) {
	store := NewConfigStore(t.TempDir() + "/config.json")
	a := NewAPI(store, t.TempDir())
	h := a.Handler()
	for _, path := range []string{"/v1/keenetic/integration-plan", "/v1/hardware/evidence"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		var decoded map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(w.Body.String(), `"activation_allowed":true`) || strings.Contains(w.Body.String(), `"changes_applied":true`) {
			t.Fatal("read-only contract violated")
		}
		post := httptest.NewRecorder()
		h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if post.Code != http.StatusMethodNotAllowed {
			t.Fatalf("unsafe method allowed on %s: %d", path, post.Code)
		}
	}
}
