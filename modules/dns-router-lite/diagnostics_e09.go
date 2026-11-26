package lite

import (
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

var e09AuditLock sync.Mutex

// Read-only diagnostic route; never calls an upstream, Gate mutation or apply.
// The saved and active snapshots are separate observations, not a transaction.
func (m *Manager) servePolicyAuditE09(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		w.WriteHeader(405)
		return
	}
	if len(r.URL.RawQuery) > 1024 {
		jsonResponse(w, 400, map[string]string{"error": "INVALID_QUERY"})
		return
	}
	// URL.Query silently discards malformed pairs; ParseQuery makes that a failure.
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		jsonResponse(w, 400, map[string]string{"error": "INVALID_QUERY"})
		return
	}
	for k, v := range q {
		if (k != "domain" && k != "mac") || len(v) != 1 {
			jsonResponse(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
	}
	name, mac := q.Get("domain"), q.Get("mac")
	if (q.Has("domain") && (len(name) == 0 || len(name) > 253)) || len(mac) > 17 || (!q.Has("domain") && q.Has("mac")) {
		jsonResponse(w, 400, map[string]string{"error": "INVALID_QUERY"})
		return
	}
	// Only one expensive diagnostic request per manager; do not block lifecycle.
	if !e09AuditLock.TryLock() {
		jsonResponse(w, 429, map[string]string{"error": "AUDIT_BUSY"})
		return
	}
	defer e09AuditLock.Unlock()
	if m.Store == nil {
		jsonResponse(w, 503, map[string]string{"error": "SAVED_POLICY_UNAVAILABLE"})
		return
	}
	saved, rev, err := m.Store.Load()
	if err != nil {
		jsonResponse(w, 503, map[string]string{"error": "SAVED_POLICY_UNAVAILABLE"})
		return
	}
	m.mu.Lock()
	active := m.engine != nil
	applied, appliedRev := m.applied, m.revision
	var engine *dr.Engine
	var cache map[string]any
	var requests, hits, failures *uint64
	var listenerV6 *bool
	if active {
		engine = m.engine.policy
		cache = m.engine.policyCache.Stats()
		a, b, c := m.engine.requests.Load(), m.engine.cacheHits.Load(), m.engine.failures.Load()
		requests, hits, failures = &a, &b, &c
		v := m.engine.listenV6
		listenerV6 = &v
	}
	var coverage any
	if m.Gate != nil {
		coverage = m.Gate.Status()["coverage"]
	}
	m.mu.Unlock()
	now := time.Now().UTC()
	out := map[string]any{"schema_version": 1, "observed_at": now, "saved_revision": rev, "active": active, "applied_revision": appliedRev, "saved": dr.AuditPolicyE09(saved, now), "applied": nil, "same_revision": active && rev == appliedRev, "policy_cache": cache, "requests": requests, "cache_hits": hits, "failures": failures, "listener_ipv6": listenerV6, "gate_coverage": coverage, "dry_run": true}
	if active {
		out["applied"] = dr.AuditPolicyE09(applied, now)
	}
	if name != "" {
		p, e := dr.Compile(saved)
		if e != nil {
			jsonResponse(w, 503, map[string]string{"error": "SAVED_POLICY_UNAVAILABLE"})
			return
		}
		view, e := p.ExplainE09(name, strings.ToLower(mac))
		if e != nil {
			jsonResponse(w, 400, map[string]string{"error": e.Error()})
			return
		}
		out["saved_explain"] = view
		if active {
			v, e := engine.ExplainE09(name, strings.ToLower(mac))
			if e != nil {
				jsonResponse(w, 409, map[string]string{"error": e.Error()})
				return
			}
			out["applied_explain"] = v
		}
	}
	jsonResponse(w, 200, out)
}
