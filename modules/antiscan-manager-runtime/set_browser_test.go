package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseAntiscanSetLimit(t *testing.T) {
	if got, err := parseAntiscanSetLimit(""); err != nil || got != 200 {
		t.Fatalf("default limit=%d err=%v", got, err)
	}
	if got, err := parseAntiscanSetLimit("500"); err != nil || got != 500 {
		t.Fatalf("max limit=%d err=%v", got, err)
	}
	for _, value := range []string{"0", "501", "nope"} {
		if _, err := parseAntiscanSetLimit(value); err == nil {
			t.Fatalf("invalid limit %q accepted", value)
		}
	}
}

func TestParseAntiscanSetList(t *testing.T) {
	fixture := `Name: ascn_ips
Type: hash:ip
Header: family inet hashsize 1024 maxelem 65536 timeout 864000 counters
Size in memory: 544
References: 1
Number of entries: 3
Members:
198.51.100.7 timeout 400 packets 8 bytes 960
203.0.113.9 timeout 0 packets 2 bytes 120
192.0.2.44 timeout 15
`

	entries, count, known, truncated, err := parseAntiscanSetList(strings.NewReader(fixture), 2)
	if err != nil {
		t.Fatal(err)
	}
	if !known || count != 3 || !truncated {
		t.Fatalf("count=%d known=%v truncated=%v", count, known, truncated)
	}
	if len(entries) != 2 {
		t.Fatalf("entries=%d", len(entries))
	}
	if entries[0].Value != "198.51.100.7" || !entries[0].TimeoutKnown || entries[0].TimeoutSeconds != 400 {
		t.Fatalf("entry0=%+v", entries[0])
	}
	if !entries[0].PacketsKnown || entries[0].Packets != 8 || !entries[0].BytesKnown || entries[0].Bytes != 960 {
		t.Fatalf("entry0 counters=%+v", entries[0])
	}
}

func TestParseAntiscanSetListSubnet(t *testing.T) {
	fixture := `Name: ascn_subnets
Type: hash:net
Number of entries: 1
Members:
100.64.10.0/24 timeout 3600
`
	entries, _, _, truncated, err := parseAntiscanSetList(strings.NewReader(fixture), 20)
	if err != nil {
		t.Fatal(err)
	}
	if truncated || len(entries) != 1 || entries[0].Value != "100.64.10.0/24" {
		t.Fatalf("entries=%+v truncated=%v", entries, truncated)
	}
}

func TestParseAntiscanSetListCountsMembersWhenHeaderOmitted(t *testing.T) {
	fixture := `Name: ascn_ips
Type: hash:ip
Revision: 4
Header: family inet hashsize 1024 maxelem 65536 timeout 864000
Size in memory: 96
References: 3
Members:
198.51.100.7 timeout 400
203.0.113.9 timeout 300
`
	entries, count, known, truncated, err := parseAntiscanSetList(strings.NewReader(fixture), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !known || count != 2 || !truncated {
		t.Fatalf("count=%d known=%v truncated=%v entries=%+v", count, known, truncated, entries)
	}

	emptyFixture := `Name: ascn_ips
Type: hash:ip
Revision: 4
Header: family inet hashsize 1024 maxelem 65536 timeout 864000
Size in memory: 96
References: 3
Members:
`
	emptyEntries, emptyCount, emptyKnown, emptyTruncated, err := parseAntiscanSetList(strings.NewReader(emptyFixture), 20)
	if err != nil {
		t.Fatal(err)
	}
	if !emptyKnown || emptyCount != 0 || emptyTruncated || len(emptyEntries) != 0 {
		t.Fatalf("empty count=%d known=%v truncated=%v entries=%+v", emptyCount, emptyKnown, emptyTruncated, emptyEntries)
	}
}

func TestAntiscanZeroCountJSONContract(t *testing.T) {
	for name, value := range map[string]any{
		"status": antiscanSetInfo{
			Name:       "ascn_ips",
			Exists:     true,
			Count:      0,
			CountKnown: true,
		},
		"browser": antiscanSetPage{
			Name:        "ascn_ips",
			Exists:      true,
			Count:       0,
			CountKnown:  true,
			Entries:     []antiscanSetEntry{},
			MutationAPI: true,
		},
	} {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("%s marshal: %v", name, err)
		}
		text := string(data)
		if !strings.Contains(text, `"count":0`) || !strings.Contains(text, `"count_known":true`) {
			t.Fatalf("%s zero-count JSON contract missing: %s", name, text)
		}
	}
}

func TestParseAntiscanSetEntryRejectsUnexpectedData(t *testing.T) {
	if _, ok := parseAntiscanSetEntry("not-an-ip timeout 10"); ok {
		t.Fatal("unexpected non-IP member accepted")
	}
}

func TestBrowseAntiscanSetRejectsUnknownName(t *testing.T) {
	page, status := browseAntiscanSet(context.Background(), runtimeConfig{}, "ascn_not_real", 20)
	if status != 400 || page.Error == "" || !page.MutationAPI {
		t.Fatalf("status=%d page=%+v", status, page)
	}
}
