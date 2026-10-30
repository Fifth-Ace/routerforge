package dnsrouter

// P8.7: explicit, bounded observation of Keenetic's connection policies.
// ndmc is invoked only for the GET policy catalog, never for normal DNS
// forwarding, saving a configuration or opening the Native selector.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const maxNDMCPolicyBytesP87 = 128 << 10
const maxNDMCPoliciesP87 = 32

var (
	ndmcPolicyHeaderP87 = regexp.MustCompile(`^\s*policy,\s*name\s*=\s*(Policy(?:0|[1-9][0-9]{0,2})),\s*description\s*=\s*(.{0,256}):\s*$`)
	ndmcTable4P87       = regexp.MustCompile(`^\s*table4:\s*([0-9]{1,8})\s*$`)
	ndmcDefault4P87     = regexp.MustCompile(`^\s*destination:\s*0\.0\.0\.0/0\s*$`)
)

type NativePolicyP87 struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	Table4          int    `json:"table4,omitempty"`
	IPv4RouteStatus string `json:"ipv4_route_status"` // default_present, default_absent, unknown
}

type NativePolicyCatalogP87 struct {
	Mode              string            `json:"mode"`
	ReadOnly          bool              `json:"read_only"`
	Available         bool              `json:"available"`
	Policies          []NativePolicyP87 `json:"policies"`
	NDMCReadAttempted bool              `json:"ndmc_read_attempted"`
	DNSProbePerformed bool              `json:"dns_probe_performed"`
	EgressVerified    bool              `json:"egress_verified"`
	EncryptionProven  bool              `json:"encryption_proven"`
	ChangesApplied    bool              `json:"changes_applied"`
	Warnings          []string          `json:"warnings"`
}

// cappedWriterP87 refuses excessive output without retaining it in memory.
type cappedWriterP87 struct{ buf bytes.Buffer }

func (w *cappedWriterP87) Write(p []byte) (int, error) {
	if len(p) > maxNDMCPolicyBytesP87-w.buf.Len() {
		return 0, errors.New("ndmc policy output limit")
	}
	return w.buf.Write(p)
}

func readNDMCPoliciesP87(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, "ndmc", "-c", "show ip policy") // validated, no shell, read-only
	if err != nil {
		return nil, fmt.Errorf("policy inventory unavailable: %w", err)
	}
	var output cappedWriterP87
	cmd.Stdout = &output
	cmd.Stderr = io.Discard // never expose host diagnostics/secrets through API
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("policy inventory unavailable: %w", err)
	}
	return append([]byte(nil), output.buf.Bytes()...), nil
}

// parseNDMCPoliciesP87 consumes bounded CLI text rather than trusting arbitrary
// names, tables, or URLs. No command strings are ever constructed from a name.
func parseNDMCPoliciesP87(raw []byte) ([]NativePolicyP87, error) {
	if len(raw) == 0 || len(raw) > maxNDMCPolicyBytesP87 || !utf8.Valid(raw) {
		return nil, errors.New("invalid or overlarge ndmc policy output")
	}
	policies := make([]NativePolicyP87, 0)
	seen := make(map[string]bool)
	active := -1
	inRoute4 := false
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 8192)
	for scanner.Scan() {
		line := scanner.Text()
		if m := ndmcPolicyHeaderP87.FindStringSubmatch(line); m != nil {
			if seen[m[1]] || len(policies) >= maxNDMCPoliciesP87 {
				return nil, errors.New("duplicate or excessive native policies")
			}
			description := strings.TrimSpace(m[2])
			if len(description) > 256 || strings.ContainsAny(description, "\x00\r\n") {
				return nil, errors.New("invalid policy description")
			}
			seen[m[1]] = true
			policies = append(policies, NativePolicyP87{Name: m[1], Description: description, IPv4RouteStatus: "unknown"})
			active = len(policies) - 1
			inRoute4 = false
			continue
		}
		if active < 0 {
			continue
		}
		if m := ndmcTable4P87.FindStringSubmatch(line); m != nil {
			table, e := strconv.Atoi(m[1])
			if e != nil || table < 1 {
				return nil, errors.New("invalid policy route table")
			}
			policies[active].Table4 = table
			policies[active].IPv4RouteStatus = "default_absent"
			continue
		}
		field := strings.TrimSpace(line)
		if field == "route4:" {
			inRoute4 = true
			continue
		}
		if field == "route6:" || strings.HasPrefix(field, "table6:") {
			inRoute4 = false
			continue
		}
		if inRoute4 && ndmcDefault4P87.MatchString(line) && policies[active].Table4 != 0 {
			policies[active].IPv4RouteStatus = "default_present"
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("ndmc policy line exceeds limit")
	}
	// Zero policies is possible on routers with only the System default route.
	return policies, nil
}

func ObserveNativePoliciesP87(ctx context.Context, read func(context.Context) ([]byte, error)) NativePolicyCatalogP87 {
	out := NativePolicyCatalogP87{Mode: "passive-keenetic-policy-catalog", ReadOnly: true,
		Policies: []NativePolicyP87{}, Warnings: []string{}, NDMCReadAttempted: true}
	raw, err := read(ctx)
	if err != nil {
		out.Warnings = append(out.Warnings, "Keenetic policy read unavailable; policy names and routes are unknown")
		return out
	}
	policies, err := parseNDMCPoliciesP87(raw)
	if err != nil {
		out.Warnings = append(out.Warnings, "Keenetic policy output rejected; policy names and routes are unknown")
		return out
	}
	out.Policies = policies
	out.Available = true
	out.Warnings = append(out.Warnings, "default route observation is not proof of DNS reachability, egress or encryption")
	return out
}

func ReadOnlyNativePoliciesP87(ctx context.Context) NativePolicyCatalogP87 {
	return ObserveNativePoliciesP87(ctx, readNDMCPoliciesP87)
}
