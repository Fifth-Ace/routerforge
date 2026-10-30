package dnsrouter

// DPR2-P8.2: bounded, read-only Keenetic DNS proxy inventory.
// This is an observation contract, NOT permission to use a stubby socket or
// to change ndnproxy, interface policies, DNS profiles, listeners or DNS 53.

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxNativeConfigP82 = 64 << 10
const maxNativeEndpointsP82 = 128
const maxNativePoliciesP82 = 32

var nativePolicyFileP82 = regexp.MustCompile(`^ndnproxy_Policy([0-9]{1,3})\.conf$`)

type NativeEndpointP82 struct {
	Address      string `json:"address"`
	Domain       string `json:"domain"`
	UpstreamHint string `json:"upstream_hint,omitempty"`
}

type NativeInstanceP82 struct {
	Name      string              `json:"name"`
	Source    string              `json:"source"`
	Endpoints []NativeEndpointP82 `json:"endpoints"`
	Valid     bool                `json:"valid"`
	Errors    []string            `json:"errors"`
}

type NativeDNSInventoryP82 struct {
	Mode              string              `json:"mode"`
	Instances         []NativeInstanceP82 `json:"instances"`
	ParsedInstances   int                 `json:"parsed_instances"`
	LocalEndpoints    int                 `json:"local_endpoints"`
	Available         bool                `json:"available"`
	ReuseVerified     bool                `json:"reuse_verified"`
	ActivationAllowed bool                `json:"activation_allowed"`
	NDMCInvoked       bool                `json:"ndmc_invoked"`
	ChangesApplied    bool                `json:"changes_applied"`
	Warnings          []string            `json:"warnings"`
}

func parseNativeConfigP82(name, source string, raw []byte) NativeInstanceP82 {
	out := NativeInstanceP82{Name: name, Source: source, Endpoints: []NativeEndpointP82{}, Errors: []string{}}
	seen := map[string]bool{}
	s := bufio.NewScanner(strings.NewReader(string(raw)))
	s.Buffer(make([]byte, 2048), 8192)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(line, "dns_server") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 || parts[0] != "dns_server" || parts[1] != "=" {
			out.Errors = append(out.Errors, "malformed dns_server entry")
			continue
		}
		host, port, err := net.SplitHostPort(parts[2])
		n, e := strconv.Atoi(port)
		if err != nil || e != nil || host != "127.0.0.1" || n < 1025 || n > 65535 {
			out.Errors = append(out.Errors, "non-loopback or invalid local endpoint")
			continue
		}
		domain := parts[3]
		if domain != "." {
			normalized, e := normalizedDomain(domain)
			if e != nil || normalized != domain {
				out.Errors = append(out.Errors, "invalid native suffix")
				continue
			}
		}
		if seen[parts[2]+"|"+domain] {
			out.Errors = append(out.Errors, "duplicate native DNS entry")
			continue
		}
		if len(out.Endpoints) >= maxNativeEndpointsP82 {
			out.Errors = append(out.Errors, "too many local endpoints")
			break
		}
		seen[parts[2]+"|"+domain] = true
		ep := NativeEndpointP82{Address: parts[2], Domain: domain}
		// Hints originate from native comments, not from RouterForge config.
		// Sanitize them: never expose arbitrary config lines to the API.
		if len(parts) >= 6 && parts[4] == "#" {
			hint := parts[5]
			if len(hint) <= 120 && strings.IndexFunc(hint, func(r rune) bool {
				return r < 33 || r > 126
			}) < 0 {
				ep.UpstreamHint = hint
			}
		}
		out.Endpoints = append(out.Endpoints, ep)
	}
	if err := s.Err(); err != nil {
		out.Errors = append(out.Errors, "configuration line too long")
	}
	if len(out.Endpoints) == 0 {
		out.Errors = append(out.Errors, "no local DNS proxy endpoints")
	}
	out.Valid = len(out.Errors) == 0
	return out
}

// ReadOnlyNativeDNSInventoryP82 inspects only known ndnproxy configuration
// filenames. It does not open sockets, invoke ndmc, query DNS or change files.
// Local ports are transient observations, never trusted forwarding targets.
func ReadOnlyNativeDNSInventoryP82(dir string) NativeDNSInventoryP82 {
	out := NativeDNSInventoryP82{
		Mode: "read-only-native-dns", Instances: []NativeInstanceP82{},
		Warnings: []string{"local stubby ports may change after DNS profile or firmware updates", "policy-specific egress and actual UDP/TCP delivery not established", "native inventory alone never permits DNS takeover"},
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		out.Warnings = append(out.Warnings, "native configuration directory unavailable")
		return out
	}
	names := []string{"ndnproxymain.conf"}
	for _, entry := range entries {
		if nativePolicyFileP82.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	if len(names) > maxNativePoliciesP82+1 {
		out.Warnings = append(out.Warnings, "too many policy configurations; inventory rejected")
		return out
	}
	sort.Strings(names[1:])
	globallySeen := map[string]string{}
	for _, filename := range names {
		name := "System"
		if filename != "ndnproxymain.conf" {
			name = strings.TrimSuffix(strings.TrimPrefix(filename, "ndnproxy_"), ".conf")
		}
		instance := NativeInstanceP82{Name: name, Source: filename, Endpoints: []NativeEndpointP82{}, Errors: []string{}}
		pathname := filepath.Join(dir, filename)
		info, statErr := os.Lstat(pathname)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > maxNativeConfigP82 {
			instance.Errors = append(instance.Errors, "native configuration missing, unsafe or too large")
		} else {
			data, readErr := os.ReadFile(pathname)
			if readErr != nil || len(data) > maxNativeConfigP82 {
				instance.Errors = append(instance.Errors, "native configuration unreadable")
			} else {
				instance = parseNativeConfigP82(name, filename, data)
			}
		}
		for _, ep := range instance.Endpoints {
			if previous, ok := globallySeen[ep.Address]; ok {
				instance.Errors = append(instance.Errors, "local port reused by "+previous)
				instance.Valid = false
			} else {
				globallySeen[ep.Address] = name
			}
		}
		if instance.Valid {
			out.ParsedInstances++
		}
		out.LocalEndpoints += len(instance.Endpoints)
		if !instance.Valid {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s: inventory incomplete or invalid", name))
		}
		out.Instances = append(out.Instances, instance)
	}
	out.Available = len(out.Instances) > 0 && out.ParsedInstances == len(out.Instances) && out.LocalEndpoints > 0
	// The above deliberately never sets ReuseVerified or ActivationAllowed.
	return out
}
