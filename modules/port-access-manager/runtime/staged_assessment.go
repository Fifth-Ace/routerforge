package main

import (
	"net/http"
	"strings"
)

// stagedAssessmentHandler binds persisted disabled rules to their immutable
// dry-run evidence. It does not execute any commands or authorize deployment.
func stagedAssessmentHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			jsonReply(w, http.StatusMethodNotAllowed, map[string]string{"error": "read-only assessment"})
			return
		}
		if len(r.URL.RawQuery) > 128 || len(r.URL.Query()) != 1 || len(r.URL.Query()["id"]) != 1 {
			jsonReply(w, http.StatusBadRequest, map[string]string{"error": "exactly one rule id is required"})
			return
		}
		id := r.URL.Query().Get("id")
		if !ruleIDPattern.MatchString(id) || strings.TrimSpace(id) != id {
			jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid rule id"})
			return
		}
		rulesMu.Lock()
		defer rulesMu.Unlock()
		stored, err := readManagedRules(path)
		if err != nil {
			jsonReply(w, http.StatusInternalServerError, map[string]string{"error": "staged rule store unavailable"})
			return
		}
		for _, rule := range stored.Rules {
			if rule.ID != id {
				continue
			}
			options := KnockOptions{Sequence: rule.Sequence, Target: rule.Target, WindowSeconds: rule.WindowSeconds, AccessSeconds: rule.AccessSeconds}
			plan, err := buildKnockRules(options)
			if err != nil || !plan.DeploymentBlocked || plan.Applied || plan.Hooked || plan.StrictSequenceVerified || rule.Enabled {
				jsonReply(w, http.StatusConflict, map[string]string{"error": "unsafe staged rule"})
				return
			}
			hooks, unhooks, err := buildKnockHook(rule.WAN, rule.Target)
			if err != nil {
				jsonReply(w, http.StatusConflict, map[string]string{"error": "invalid hook preview"})
				return
			}
			jsonReply(w, http.StatusOK, map[string]any{
				"rule_id": id, "mode": "dry-run-only", "ready_for_apply": false,
				"firewall_mutation": false, "strict_sequence_verified": false,
				"deployment_blocked": true, "deployment_blockers": plan.DeploymentBlockers,
				"commands": plan.Commands, "hook_preview": hooks,
				"unhook_preview": unhooks, "rollback_preview": buildKnockRollback(),
				"evidence": "preview only; NDM and hardware verification required",
			})
			return
		}
		jsonReply(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
	}
}
