package main

import (
	"strings"
	"testing"
)

func TestNF1B1StaticSemanticReports(t *testing.T) {
	cases := []struct {
		name, content, severity, code string
		ok                            bool
	}{
		{"normal", `TCP_PORTS=80,443
NFQWS_ARGS='--filter-tcp=443 --lua-desync=multisplit:pos=1'`, "", "", true},
		{"v1_legacy", `NFQWS_ARGS_CUSTOM='--filter-tcp=443 --dpi-desync=fake'`, "error", "LEGACY_NFQWS_V1", false},
		{"port_overflow", `TCP_PORTS=80,70000`, "error", "INVALID_PORT_RANGE", false},
		{"filter_port", `NFQWS_ARGS='--filter-udp=99999'`, "error", "INVALID_FILTER_PORT", false},
		{"empty_hostlist", `NFQWS_ARGS='--hostlist='`, "error", "EMPTY_OPTION_VALUE", false},
		{"unknown_payload", `NFQWS_ARGS_CUSTOM='--filter-tcp=443 --payload=unknown --lua-desync=fake:blob=hello'`, "warning", "UNKNOWN_PAYLOAD_LUA", true},
		{"dynamic_unverified", "NFQWS_ARGS=\"$(touch /tmp/unsafe-marker)\"\n", "info", "SHELL_DYNAMIC_UNVERIFIED", true},
		{"comment_is_not_flag", "NFQWS_ARGS='# --dpi-desync=fake\n--filter-tcp=443'", "", "", true},
		{"multiline", "NFQWS_ARGS=\"--filter-tcp=443\\\n--lua-desync=multisplit:pos=1\"", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nf1bConfigSemantics(tc.content)
			if got.OK != tc.ok {
				t.Fatalf("OK=%v want=%v issues=%+v", got.OK, tc.ok, got.Issues)
			}
			if tc.code == "" {
				if len(got.Issues) > 0 {
					t.Fatalf("unexpected issues %+v", got.Issues)
				}
				return
			}
			found := false
			for _, issue := range got.Issues {
				if issue.Code == tc.code && issue.Severity == tc.severity {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %s / %s in %+v", tc.severity, tc.code, got.Issues)
			}
		})
	}
}

func TestNF1B1DoesNotExecuteContent(t *testing.T) {
	report := nf1bConfigSemantics("NFQWS_ARGS=\"$(touch /tmp/nf1b1-should-never-exist)\"\n")
	if !report.OK || report.Unverified == 0 || !strings.Contains(report.Issues[0].Code, "UNVERIFIED") {
		t.Fatalf("unexpected report: %+v", report)
	}
}
