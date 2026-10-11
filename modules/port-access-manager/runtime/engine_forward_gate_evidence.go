package main

import (
	"errors"
	"fmt"
	"strings"
)

// forwardGateEvidence is read-only evidence about an existing Keenetic forwarding
// path. It cannot prove that a service is protected and must never be used as an
// authorization to mutate the firewall.
type forwardGateEvidence struct {
	HasDNATAccept              bool   `json:"has_dnat_accept"`
	HasKeeneticForward         bool   `json:"has_ndm_forward"`
	HasExistingRouterForgeGate bool   `json:"has_existing_routerforge_gate"`
	SuitableForDirectApply     bool   `json:"suitable_for_direct_apply"`
	Reason                     string `json:"reason"`
}

// inspectForwardGateRules accepts the output of `iptables -t filter -S FORWARD`.
// It conservatively declines every direct application: an isolated preview must
// never be mistaken for a validated hardware transaction.
func inspectForwardGateRules(rules string) (forwardGateEvidence, error) {
	var e forwardGateEvidence
	if strings.TrimSpace(rules) == "" {
		return e, errors.New("empty FORWARD snapshot")
	}
	if !strings.Contains(rules, "-P FORWARD ") {
		return e, errors.New("missing FORWARD policy")
	}
	for _, line := range strings.Split(rules, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "-A FORWARD ") {
			continue
		}
		if strings.Contains(line, " -j _NDM_FORWARD") {
			e.HasKeeneticForward = true
		}
		if strings.Contains(line, " --ctstate DNAT ") && strings.HasSuffix(line, " -j ACCEPT") {
			e.HasDNATAccept = true
		}
		if strings.Contains(line, "-j RF_PA_") {
			e.HasExistingRouterForgeGate = true
		}
	}
	e.Reason = "independent early FORWARD gate, ownership checks, watchdog and real negative connection test are required"
	if !e.HasKeeneticForward {
		e.Reason = "NDM FORWARD entry missing; platform layout is not verified"
	}
	if e.HasExistingRouterForgeGate {
		e.Reason = "previous RouterForge gate present; ownership and rollback must be checked"
	}
	if e.HasDNATAccept {
		e.Reason = fmt.Sprintf("DNAT traffic is accepted by FORWARD; %s", e.Reason)
	}
	return e, nil
}
