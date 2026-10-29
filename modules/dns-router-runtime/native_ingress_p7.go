package dnsrouter

// DPR2-P7: conservative read-only native ingress audit.
// This module never executes ndmc, changes a route, or authorizes activation.
// Only suffix registrations can be advertised to Keenetic's native DNS proxy;
// rules that do not fall under those registrations cannot reach this forwarder.

import (
	"fmt"
	"sort"
	"strings"
)

const ingressAuditMaxSamplesP7 = 48

type IngressRuleSampleP7 struct {
	RuleID  string `json:"rule_id"`
	Domain  string `json:"domain"`
	Match   string `json:"match"`
	GroupID string `json:"group_id,omitempty"`
	Zone    string `json:"zone,omitempty"`
	Status  string `json:"status"` // covered, unmapped, shadowed, invalid
	Detail  string `json:"detail,omitempty"`
}

type NativeIngressAuditP7 struct {
	Mode                           string                `json:"mode"`
	SafeToApply                    bool                  `json:"safe_to_apply"`
	ChangesApplied                 bool                  `json:"changes_applied"`
	NDMCInvoked                    bool                  `json:"ndmc_invoked"`
	InventoryScope                 string                `json:"inventory_scope"`
	InventoryChecked               bool                  `json:"inventory_checked"`
	Listener                       string                `json:"listener,omitempty"`
	PlannedZones                   []DomainSplitZoneL    `json:"planned_zones"`
	RuleCount                      int                   `json:"rule_count"`
	Covered                        int                   `json:"covered"`
	Unmapped                       int                   `json:"unmapped"`
	Shadowed                       int                   `json:"shadowed"`
	Invalid                        int                   `json:"invalid"`
	Samples                        []IngressRuleSampleP7 `json:"samples"`
	Conflicts                      []string              `json:"conflicts"`
	Warnings                       []string              `json:"warnings"`
	ExistingTransactionZoneLimit   int                   `json:"existing_transaction_zone_limit"`
	WithinExistingTransactionLimit bool                  `json:"within_existing_transaction_limit"`
	AllRulesReachable              bool                  `json:"all_rules_reachable"`
}

func matchingIngressZoneP7(q string, zones []DomainSplitZoneL) string {
	var best string
	for _, z := range zones {
		d := z.Domain
		if q == d || strings.HasSuffix(q, "."+d) {
			if len(d) > len(best) {
				best = d
			}
		}
	}
	return best
}

// BuildNativeIngressAuditP7 never permits apply. Its optional snapshot must be
// a canonical read-only `show ip name-server` inventory. It cannot establish
// DoT/DoH profile precedence or prove end-to-end packet delivery.
func BuildNativeIngressAuditP7(cfg Config, nativeSnapshot string) NativeIngressAuditP7 {
	plan := BuildDomainSplitPlanL(cfg)
	out := NativeIngressAuditP7{
		Mode: "read-only", SafeToApply: false, ChangesApplied: false,
		NDMCInvoked: false, InventoryScope: "show ip name-server only; DoT/DoH profiles not enumerated",
		Listener: plan.Listener, PlannedZones: plan.Zones,
		RuleCount: len(cfg.Rules), Samples: []IngressRuleSampleP7{},
		Conflicts: []string{}, Warnings: append([]string{}, plan.Warnings...),
		ExistingTransactionZoneLimit:   managedMaxZonesN,
		WithinExistingTransactionLimit: len(plan.Zones) > 0 && len(plan.Zones) <= managedMaxZonesN,
	}
	if nativeSnapshot != "" {
		out.InventoryChecked = true
		conflicts, err := conflictingZonesN(nativeSnapshot, plan.Zones)
		if err != nil {
			out.Warnings = append(out.Warnings, "native inventory invalid: "+err.Error())
			out.Invalid++
		} else {
			out.Conflicts = conflicts
		}
	} else {
		out.Warnings = append(out.Warnings, "native inventory not read: run an explicit live read-only check")
	}
	engine, err := Compile(cfg)
	if err != nil {
		out.Warnings = append(out.Warnings, "configuration does not compile: "+err.Error())
		out.Invalid += len(cfg.Rules)
		return out
	}
	rules := append([]Rule(nil), cfg.Rules...)
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Domain != rules[j].Domain {
			return rules[i].Domain < rules[j].Domain
		}
		return rules[i].ID < rules[j].ID
	})
	for _, r := range rules {
		query := r.Domain
		if r.Match == "subdomains" {
			query = "p7-check." + query
		}
		sample := IngressRuleSampleP7{RuleID: r.ID, Domain: r.Domain, Match: r.Match, Status: "unmapped"}
		if len(query) > 253 {
			sample.Status = "invalid"
			sample.Detail = "probe name exceeds DNS name length"
			out.Invalid++
		} else {
			sample.Zone = matchingIngressZoneP7(query, plan.Zones)
			if sample.Zone == "" {
				out.Unmapped++
				sample.Detail = "no native suffix registration covers this policy query"
			} else {
				decision, evalErr := engine.Evaluate(query)
				if evalErr != nil {
					sample.Status = "invalid"
					sample.Detail = evalErr.Error()
					out.Invalid++
				} else if decision.RuleID != r.ID || !decision.Matched || decision.Source != "policy" {
					sample.Status = "shadowed"
					sample.Detail = "effective rule: " + decision.RuleID
					out.Shadowed++
				} else if _, chainErr := engine.UpstreamChainForDecision(decision); chainErr != nil {
					sample.Status = "invalid"
					sample.Detail = chainErr.Error()
					out.Invalid++
				} else {
					sample.Status = "covered"
					sample.GroupID = decision.GroupID
					out.Covered++
				}
			}
		}
		if sample.Status != "covered" || len(out.Samples) < 8 {
			if len(out.Samples) < ingressAuditMaxSamplesP7 {
				out.Samples = append(out.Samples, sample)
			}
		}
	}
	if !out.WithinExistingTransactionLimit {
		out.Warnings = append(out.Warnings, fmt.Sprintf("existing bounded DSTest transaction supports 1..%d zones; no limit changes in P7", managedMaxZonesN))
	}
	if len(out.Conflicts) > 0 {
		out.Warnings = append(out.Warnings, "existing Keenetic DNS zones overlap the planned registrations; no automatic takeover")
	}
	if out.Unmapped > 0 || out.Shadowed > 0 || out.Invalid > 0 {
		out.Warnings = append(out.Warnings, "the stored policy set is not fully reachable via the current native zone plan")
	}
	out.Warnings = append(out.Warnings, "DoT/DoH profile ordering and actual packet delivery are NOT proven by this preview")
	out.AllRulesReachable = out.RuleCount > 0 && out.Unmapped == 0 && out.Shadowed == 0 && out.Invalid == 0 && plan.Ready
	// This flag deliberately remains false even if all rules appear covered:
	// live traffic verification and explicit DSTest-only transaction remain mandatory.
	return out
}
