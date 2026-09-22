package main

import "testing"

func TestP26DirectSelectorAutoPoolUsesCorpusWithoutIntelligenceLayers(t *testing.T) {
	enabled := true
	req := v2SelectorRequest{
		Mode:       "thorough",
		ServerName: "example.com",
		AutoPool:   &enabled,
	}
	meta, err := populateV2SelectorCandidates(&req)
	if err != nil {
		t.Fatal(err)
	}
	if !meta.Enabled {
		t.Fatal("direct auto pool unexpectedly disabled")
	}
	if len(req.Candidates) == 0 {
		t.Fatal("direct auto pool returned no corpus candidates")
	}
	if len(req.Candidates) > 32 {
		t.Fatalf("direct auto pool exceeded thorough candidate budget: %d", len(req.Candidates))
	}
	hasOther := false
	for _, candidate := range req.Candidates {
		switch candidate.Source {
		case "synthesized", "mutated", "memory":
			t.Fatalf("intelligence-layer source leaked into direct selector pool: %s", candidate.Source)
		case "other":
			hasOther = true
		}
	}
	if !hasOther {
		t.Fatal("neutral other-strategy corpus did not reach direct selector pool")
	}
	if meta.SynthesizedCandidates != 0 || meta.MemoryCandidates != 0 {
		t.Fatalf("direct selector metadata still reports intelligence candidates: %+v", meta)
	}
}

func TestP26DirectSelectorModeIsSinglePassPerCandidate(t *testing.T) {
	for _, name := range []string{"fast", "normal", "thorough"} {
		mode, err := v2SelectorMode(name)
		if err != nil {
			t.Fatal(err)
		}
		mode.Attempts = 1
		if mode.Attempts != 1 {
			t.Fatalf("%s direct selector attempts=%d want 1", name, mode.Attempts)
		}
	}
}
