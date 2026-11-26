package dnsrouter

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
)

// Recovery is saved-policy-only. Reject any change to listener/service/advanced
// options and all URL/auto-refresh lists in this first native adapter. The
// existing managed ownership gate and ConfigStore CAS remain authoritative.
func recoveryPolicyCompatible(current, desired Config) bool {
	if !reflect.DeepEqual(current.Listener, desired.Listener) || !reflect.DeepEqual(current.Service, desired.Service) || !reflect.DeepEqual(current.Advanced, desired.Advanced) {
		return false
	}
	for _, cfg := range []Config{current, desired} {
		for _, l := range cfg.Lists {
			if (l.Kind != "paste" && l.Kind != "file") || l.URL != "" || l.AutoUpdateMinutes != 0 {
				return false
			}
		}
	}
	return true
}
func (a *API) registerRecoveryPolicy(mux *http.ServeMux) {
	for _, commit := range []bool{false, true} {
		commit := commit
		path, method, action := "/v1/recovery-policy-preview", http.MethodPost, "recovery-policy-preview"
		if commit {
			path, method, action = "/v1/recovery-policy-save", http.MethodPut, "recovery-policy-save"
		}
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != method {
				w.Header().Set("Allow", method)
				w.WriteHeader(405)
				return
			}
			if !requireMutation(w, r, action) {
				return
			}
			if a.LedgerO != nil {
				owned, _, e := a.LedgerO.Load()
				if e != nil || owned != nil {
					fail(w, 409, errors.New("RECOVERY_MANAGED_OWNERSHIP"))
					return
				}
			}
			var desired Config
			if e := decodeBody(r, &desired, maxAPIBytes); e != nil {
				fail(w, 400, errors.New("RECOVERY_INVALID_CONFIG"))
				return
			}
			if _, e := Compile(desired); e != nil {
				fail(w, 400, errors.New("RECOVERY_INVALID_CONFIG"))
				return
			}
			current, rev, e := a.Store.Load()
			if e != nil {
				fail(w, 409, errors.New("RECOVERY_CURRENT_UNAVAILABLE"))
				return
			}
			if !recoveryPolicyCompatible(current, desired) {
				fail(w, 409, errors.New("RECOVERY_INCOMPATIBLE_SAVED_POLICY"))
				return
			}
			expected := strings.Trim(r.Header.Get("If-Match"), "\"")
			if expected == "" || expected != rev {
				fail(w, 409, ErrStale)
				return
			}
			target := revision(desired)
			if !commit {
				respondJSON(w, 200, map[string]any{"current_revision": rev, "target_revision": target, "changed": rev != target, "upstreams": len(desired.Upstreams), "rules": len(desired.Rules), "lists": len(desired.Lists), "runtime": "NOT_APPLIED"})
				return
			}
			if r.Header.Get("X-RouterForge-Confirm") != "RECOVERY_POLICY_SAVE" || rev == target {
				fail(w, 409, errors.New("RECOVERY_CONFIRMATION_OR_NO_CHANGE"))
				return
			}
			saved, e := a.Store.Save(desired, expected)
			if e != nil || saved != target {
				fail(w, 409, errors.New("RECOVERY_SAVE_UNCERTAIN_INSPECT"))
				return
			}
			_, observed, e := a.Store.Load()
			if e != nil || observed != target {
				fail(w, 409, errors.New("RECOVERY_READBACK_UNCERTAIN"))
				return
			}
			respondJSON(w, 200, map[string]any{"saved": true, "revision": observed, "runtime": "NOT_APPLIED"})
		})
	}
}
