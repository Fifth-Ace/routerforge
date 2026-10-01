package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const antiscanSchedulerDefaultFixture = `*/1 * * * * /opt/etc/init.d/S99ascn read_candidates &
*/2 * * * * /opt/etc/init.d/S99ascn read_ndm_ipsets &
0 0 */5 * * /opt/etc/init.d/S99ascn save_ipsets &
0 5 */15 * * /opt/etc/init.d/S99ascn update_ipsets geo &
`

func TestParseAntiscanSchedulerPinnedDefault(t *testing.T) {
	tasks, lines, problems := parseAntiscanSchedulerSource(antiscanSchedulerDefaultFixture)
	if len(problems) != 0 {
		t.Fatalf("problems=%v", problems)
	}
	if len(tasks) != 4 || len(lines) != 4 {
		t.Fatalf("tasks=%d lines=%d", len(tasks), len(lines))
	}
	if tasks["read_candidates"] != "*/1 * * * *" {
		t.Fatalf("read_candidates=%q", tasks["read_candidates"])
	}
	if tasks["update_ipsets geo"] != "0 5 */15 * *" {
		t.Fatalf("geo=%q", tasks["update_ipsets geo"])
	}
	if tasks["retry_load_geo"] != "" {
		t.Fatalf("unexpected retry task=%q", tasks["retry_load_geo"])
	}
}

func TestParseAntiscanSchedulerRejectsUnsafeOrDuplicateLines(t *testing.T) {
	for name, fixture := range map[string]string{
		"arbitrary-command": "* * * * * /bin/sh -c boom &\n",
		"unsupported-task":  "* * * * * /opt/etc/init.d/S99ascn token check &\n",
		"missing-ampersand": "* * * * * /opt/etc/init.d/S99ascn read_candidates\n",
		"bad-field":         "0,5 * * * * /opt/etc/init.d/S99ascn read_candidates &\n",
		"duplicate": antiscanSchedulerDefaultFixture +
			"*/3 * * * * /opt/etc/init.d/S99ascn read_candidates &\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, problems := parseAntiscanSchedulerSource(fixture)
			if len(problems) == 0 {
				t.Fatal("unsafe fixture accepted")
			}
		})
	}
}

func TestNormalizeAntiscanSchedulerRequestContract(t *testing.T) {
	valid := []antiscanSchedulerTaskRequest{
		{Task: "read_candidates", Enabled: true, Schedule: "*/1 * * * *"},
		{Task: "read_ndm_ipsets", Enabled: true, Schedule: "*/2 * * * *"},
		{Task: "save_ipsets", Enabled: true, Schedule: "0 0 */5 * *"},
		{Task: "update_ipsets geo", Enabled: true, Schedule: "0 5 */15 * *"},
	}
	if _, err := normalizeAntiscanSchedulerRequest(valid); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}

	withRetry := append([]antiscanSchedulerTaskRequest(nil), valid...)
	withRetry[0].Task = "retry_load_geo"
	if _, err := normalizeAntiscanSchedulerRequest(withRetry); err == nil {
		t.Fatal("automatic retry task must not be user-managed")
	}

	allDisabled := append([]antiscanSchedulerTaskRequest(nil), valid...)
	for i := range allDisabled {
		allDisabled[i].Enabled = false
	}
	if _, err := normalizeAntiscanSchedulerRequest(allDisabled); err == nil {
		t.Fatal("empty managed schedule accepted")
	}
}

func TestRenderAntiscanSchedulerPreservesCommentsAndAutoRetry(t *testing.T) {
	current := "# keep me\n" + antiscanSchedulerDefaultFixture +
		"0 */1 * * * /opt/etc/init.d/S99ascn retry_load_geo &\n"
	requested, err := normalizeAntiscanSchedulerRequest([]antiscanSchedulerTaskRequest{
		{Task: "read_candidates", Enabled: true, Schedule: "*/3 * * * *"},
		{Task: "read_ndm_ipsets", Enabled: false, Schedule: "*/2 * * * *"},
		{Task: "save_ipsets", Enabled: true, Schedule: "0 0 */5 * *"},
		{Task: "update_ipsets geo", Enabled: true, Schedule: "0 5 */15 * *"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderAntiscanSchedulerSource(current, requested)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "# keep me") {
		t.Fatal("comment was not preserved")
	}
	if !strings.Contains(rendered, "*/3 * * * * /opt/etc/init.d/S99ascn read_candidates &") {
		t.Fatal("updated candidate schedule missing")
	}
	if strings.Contains(rendered, "read_ndm_ipsets") {
		t.Fatal("disabled task still present")
	}
	if !strings.Contains(rendered, "0 */1 * * * /opt/etc/init.d/S99ascn retry_load_geo &") {
		t.Fatal("automatic retry task was not preserved")
	}
}

func TestApplyAntiscanSchedulerUpdatesSourceAndActiveCron(t *testing.T) {
	cfg, active := fakeAntiscanSchedulerConfig(t, false)
	request := antiscanSchedulerRequest{
		BaseSHA256: antiscanSchedulerSHA([]byte(antiscanSchedulerDefaultFixture)),
		Tasks: []antiscanSchedulerTaskRequest{
			{Task: "read_candidates", Enabled: true, Schedule: "*/3 * * * *"},
			{Task: "read_ndm_ipsets", Enabled: false, Schedule: "*/2 * * * *"},
			{Task: "save_ipsets", Enabled: true, Schedule: "0 0 */5 * *"},
			{Task: "update_ipsets geo", Enabled: true, Schedule: "0 5 */15 * *"},
		},
	}

	result, status, err := applyAntiscanScheduler(context.Background(), cfg, request)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if status != 200 || !result.Changed || !result.Verified || !result.Synced || result.RollbackPerformed {
		t.Fatalf("status=%d result=%+v", status, result)
	}
	data, err := os.ReadFile(filepath.Join(cfg.AntiscanDir, "ascn_crontab.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "read_ndm_ipsets") || !strings.Contains(string(data), "*/3 * * * *") {
		t.Fatalf("unexpected source:\n%s", data)
	}
	activeData, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if string(activeData) != string(data) {
		t.Fatalf("active cron did not follow source\nsource=%q\nactive=%q", data, activeData)
	}
}

func TestApplyAntiscanSchedulerRollsBackWhenUpstreamFails(t *testing.T) {
	cfg, active := fakeAntiscanSchedulerConfig(t, true)
	original, err := os.ReadFile(filepath.Join(cfg.AntiscanDir, "ascn_crontab.conf"))
	if err != nil {
		t.Fatal(err)
	}
	request := antiscanSchedulerRequest{
		BaseSHA256: antiscanSchedulerSHA(original),
		Tasks: []antiscanSchedulerTaskRequest{
			{Task: "read_candidates", Enabled: true, Schedule: "*/4 * * * *"},
			{Task: "read_ndm_ipsets", Enabled: true, Schedule: "*/2 * * * *"},
			{Task: "save_ipsets", Enabled: true, Schedule: "0 0 */5 * *"},
			{Task: "update_ipsets geo", Enabled: true, Schedule: "0 5 */15 * *"},
		},
	}

	result, status, err := applyAntiscanScheduler(context.Background(), cfg, request)
	if err == nil || status != 409 || !result.RollbackPerformed {
		t.Fatalf("status=%d err=%v result=%+v", status, err, result)
	}
	after, err := os.ReadFile(filepath.Join(cfg.AntiscanDir, "ascn_crontab.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("source rollback mismatch\nwant=%q\ngot=%q", original, after)
	}
	activeData, err := os.ReadFile(active)
	if err != nil {
		t.Fatal(err)
	}
	if string(activeData) != string(original) {
		t.Fatalf("active cron changed on failed update\nwant=%q\ngot=%q", original, activeData)
	}
}

func fakeAntiscanSchedulerConfig(t *testing.T, failUpdate bool) (runtimeConfig, string) {
	t.Helper()
	dir := t.TempDir()
	antiscanDir := filepath.Join(dir, "antiscan")
	if err := os.MkdirAll(antiscanDir, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(antiscanDir, "ascn_crontab.conf")
	if err := os.WriteFile(source, []byte(antiscanSchedulerDefaultFixture), 0644); err != nil {
		t.Fatal(err)
	}
	active := filepath.Join(dir, "active.cron")
	if err := os.WriteFile(active, []byte(antiscanSchedulerDefaultFixture), 0644); err != nil {
		t.Fatal(err)
	}

	crontab := filepath.Join(dir, "crontab")
	crontabScript := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"-l\" ]; then cat '%s'; exit 0; fi\nexit 2\n", shellSingleQuoteTest(active))
	if err := os.WriteFile(crontab, []byte(crontabScript), 0755); err != nil {
		t.Fatal(err)
	}
	oldFind := antiscanSchedulerFindCrontab
	antiscanSchedulerFindCrontab = func() string { return crontab }
	t.Cleanup(func() { antiscanSchedulerFindCrontab = oldFind })

	initScript := filepath.Join(dir, "S99ascn")
	body := fmt.Sprintf("#!/bin/sh\n[ \"$1\" = \"update_crontab\" ] || exit 2\ncat '%s' > '%s'\nexit 0\n", shellSingleQuoteTest(source), shellSingleQuoteTest(active))
	if failUpdate {
		body = "#!/bin/sh\nexit 1\n"
	}
	if err := os.WriteFile(initScript, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return runtimeConfig{AntiscanDir: antiscanDir, InitScript: initScript}, active
}

func shellSingleQuoteTest(value string) string {
	return strings.ReplaceAll(value, "'", "'\\''")
}
