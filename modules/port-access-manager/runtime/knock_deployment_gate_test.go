package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestK4PPreviewDeploymentBlockers(t *testing.T) {
	plan, err := buildKnockRules(KnockOptions{Sequence: [3]int{41001, 41002, 41003}, Target: 2222, WindowSeconds: 30, AccessSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.DeploymentBlocked || len(plan.DeploymentBlockers) != 3 || plan.Hooked || plan.Applied || plan.StrictSequenceVerified {
		t.Fatalf("unsafe activation contract: %+v", plan)
	}
	h := routes(t.TempDir())
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/v1/knock-preview?first=41001&second=41002&third=41003&target=2222&window=30&ttl=120", nil))
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var got struct {
		DeploymentBlocked  bool     `json:"deployment_blocked"`
		DeploymentBlockers []string `json:"deployment_blockers"`
		Applied            bool     `json:"applied"`
		Hooked             bool     `json:"hooked"`
		Strict             bool     `json:"strict_sequence_verified"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.DeploymentBlocked || len(got.DeploymentBlockers) != 3 || got.Applied || got.Hooked || got.Strict {
		t.Fatalf("unsafe preview: %+v", got)
	}
}
