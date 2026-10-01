package main

import (
	"strings"
	"testing"
)

func TestPinnedAntiscanContractMatchesManagerCoverage(t *testing.T) {
	if antiscanUpstreamPinnedSHA != "f8a052b39d47c86fc3c00983a5502903a1cc2cd9" {
		t.Fatalf("unexpected pinned upstream SHA: %s", antiscanUpstreamPinnedSHA)
	}
	if antiscanUpstreamPinnedVersion != "1.10.6" {
		t.Fatalf("unexpected pinned upstream version: %s", antiscanUpstreamPinnedVersion)
	}
	if len(antiscanUpstreamConfigKeys) != 25 {
		t.Fatalf("config key count=%d", len(antiscanUpstreamConfigKeys))
	}
	if !sameStringSet(antiscanConfigKeys, antiscanUpstreamConfigKeys) {
		t.Fatalf("manager config keys differ from pinned upstream contract")
	}
	if len(antiscanUpstreamIPSets) != 11 {
		t.Fatalf("ipset count=%d", len(antiscanUpstreamIPSets))
	}
	if !sameStringSet(antiscanKnownSets, antiscanUpstreamIPSets) {
		t.Fatalf("manager ipsets differ from pinned upstream contract")
	}
}

func TestPinnedAntiscanCommandContractIncludesOperationalSurface(t *testing.T) {
	joined := "|" + strings.Join(antiscanUpstreamCommands, "|") + "|"
	for _, command := range []string{
		"restart", "flush", "token", "update_rules", "read_candidates",
		"read_ndm_ipsets", "save_ipsets", "update_ipsets", "update_crontab", "retry_load_geo",
	} {
		if !strings.Contains(joined, "|"+command+"|") {
			t.Fatalf("missing upstream command %q", command)
		}
	}
}

func TestParseAntiscanCronTasksAcceptsPinnedValidTasks(t *testing.T) {
	fixture := `*/1 * * * * /opt/etc/init.d/S99ascn read_candidates &
*/2 * * * * /opt/etc/init.d/S99ascn read_ndm_ipsets &
0 0 */5 * * /opt/etc/init.d/S99ascn save_ipsets &
0 5 */15 * * /opt/etc/init.d/S99ascn update_ipsets geo &
15 3 * * * /opt/etc/init.d/S99ascn retry_load_geo &
`
	tasks, invalid := parseAntiscanCronTasks(fixture)
	if len(invalid) != 0 {
		t.Fatalf("invalid=%v", invalid)
	}
	if len(tasks) != 5 {
		t.Fatalf("tasks=%v", tasks)
	}
}

func TestParseAntiscanCronTasksRejectsArbitraryCommand(t *testing.T) {
	_, invalid := parseAntiscanCronTasks("* * * * * /opt/etc/init.d/S99ascn rm -rf / &\n")
	if len(invalid) != 1 {
		t.Fatalf("invalid=%v", invalid)
	}
}

func TestDiagnosticOverall(t *testing.T) {
	if got := diagnosticOverall([]antiscanDiagnosticCheck{{State: "pass"}, {State: "info"}}); got != "pass" {
		t.Fatalf("pass/info overall=%q", got)
	}
	if got := diagnosticOverall([]antiscanDiagnosticCheck{{State: "pass"}, {State: "warn"}}); got != "warn" {
		t.Fatalf("warn overall=%q", got)
	}
	if got := diagnosticOverall([]antiscanDiagnosticCheck{{State: "warn"}, {State: "fail"}}); got != "fail" {
		t.Fatalf("fail overall=%q", got)
	}
}
