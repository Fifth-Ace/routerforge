package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestStagedAssessmentBindsRuleAndNeverApplies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	rule := exampleRule()
	if err := writeManagedRules(path, managedRuleFile{Schema: 1, Rules: []managedKnockRule{rule}}); err != nil {
		t.Fatal(err)
	}
	h := stagedAssessmentHandler(path)
	request := httptest.NewRequest(http.MethodGet, "/v1/staged-assessment?id="+rule.ID, nil)
	response := httptest.NewRecorder()
	h(response, request)
	if response.Code != 200 {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		RuleID   string     `json:"rule_id"`
		Ready    bool       `json:"ready_for_apply"`
		Mutation bool       `json:"firewall_mutation"`
		Blocked  bool       `json:"deployment_blocked"`
		Strict   bool       `json:"strict_sequence_verified"`
		Commands [][]string `json:"commands"`
		Rollback [][]string `json:"rollback_preview"`
		Hooks    [][]string `json:"hook_preview"`
		Unhooks  [][]string `json:"unhook_preview"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.RuleID != rule.ID || body.Ready || body.Mutation || !body.Blocked || body.Strict || len(body.Commands) == 0 || len(body.Rollback) == 0 || len(body.Hooks) != 1 || len(body.Unhooks) != 1 {
		t.Fatalf("invalid assessment: %+v", body)
	}
}

func TestStagedAssessmentRejectsWritesAndQueries(t *testing.T) {
	h := stagedAssessmentHandler(filepath.Join(t.TempDir(), "rules.json"))
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/v1/staged-assessment?id=foo", 405},
		{http.MethodGet, "/v1/staged-assessment", 400},
		{http.MethodGet, "/v1/staged-assessment?id=x&id=y", 400},
		{http.MethodGet, "/v1/staged-assessment?id=bad%20id", 400},
		{http.MethodGet, "/v1/staged-assessment?id=missing", 404},
	} {
		r := httptest.NewRecorder()
		h(r, httptest.NewRequest(tc.method, tc.path, nil))
		if r.Code != tc.want {
			t.Errorf("%s %s: got %d want %d", tc.method, tc.path, r.Code, tc.want)
		}
	}
}
