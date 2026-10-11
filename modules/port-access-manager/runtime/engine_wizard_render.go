package main

import (
	"errors"
	"fmt"
	"strings"
)

// renderedWizard describes candidate daemon configs. They are NOT applied by
// preview: packet filtering must be verified separately on the real router.
type renderedWizard struct {
	Engine               string            `json:"engine"`
	Files                map[string]string `json:"files"`
	NeedsForwardGate     bool              `json:"needs_forward_gate"`
	RequiresClientSecret bool              `json:"requires_client_secret"`
}

// renderWizard produces a non-executable, non-secret preview for human review.
// It deliberately does not manufacture an unsafe ACCEPT rule: a daemon merely
// running is not evidence that a protected port is closed before authorization.
func renderWizard(req EngineWizardRequest) (renderedWizard, error) {
	if err := validateWizard(req); err != nil {
		return renderedWizard{}, err
	}
	out := renderedWizard{Engine: req.Engine, Files: map[string]string{}, NeedsForwardGate: true}
	if req.Engine == "knockd" {
		var b strings.Builder
		fmt.Fprintf(&b, "[options]\nInterface = %s\nUseSyslog\n\n", req.Interface)
		// A single event is used intentionally: all listed destinations share
		// authorization and can be controlled through one owning gate.
		fmt.Fprintf(&b, "[routerforge-authorize]\nsequence = %d,%d,%d\nseq_timeout = %d\ntcpflags = syn\n", req.Knock[0], req.Knock[1], req.Knock[2], req.SequenceSeconds)
		b.WriteString("# start_command is intentionally unset until the RouterForge\n")
		b.WriteString("# authorization helper and FORWARD/INPUT gate pass hardware tests.\n")
		out.Files["knockd.conf.preview"] = b.String()
		return out, nil
	}
	// SPA must not expose fabricated or placeholder secret material. On actual
	// apply, generate a fresh independent encryption/HMAC key pair and send
	// client credentials only via an explicit secrets flow.
	out.RequiresClientSecret = true
	ports := make([]string, 0, len(req.Services))
	for _, s := range req.Services {
		ports = append(ports, fmt.Sprintf("tcp/%d", s.Port))
	}
	if len(ports) == 0 {
		return renderedWizard{}, errors.New("no services")
	}
	out.Files["access.conf.preview"] = "SOURCE ANY\nOPEN_PORTS " + strings.Join(ports, ",") + "\n# KEY_BASE64 and HMAC_KEY_BASE64 will be provisioned via a separate authorized operation.\n"
	return out, nil
}
