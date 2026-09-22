package main

import "testing"

func TestV2DPIPropertyProbeCatalogCompiles(t *testing.T) {
	mode, err := v2SelectorMode("thorough")
	if err != nil {
		t.Fatal(err)
	}
	_ = mode
	for _, id := range []string{benchTransportHTTPS, benchTransportHTTP, benchTransportQUIC, benchTransportSTUN} {
		transport, err := normalizeBenchTransport(id)
		if err != nil {
			t.Fatal(err)
		}
		catalog := v2DPIPropertyProbeCatalog(transport)
		if len(catalog) == 0 {
			t.Fatalf("transport=%s has no property probes", id)
		}
		seen := map[string]bool{}
		for _, spec := range catalog {
			if spec.ID == "" || seen[spec.ID] {
				t.Fatalf("transport=%s duplicate/empty probe id=%q", id, spec.ID)
			}
			seen[spec.ID] = true
			if len(spec.Families) == 0 {
				t.Fatalf("transport=%s probe=%s has no families", id, spec.ID)
			}
			profile, err := v2CustomProfileForTransport(spec.Args, "example.com", transport)
			if err != nil {
				t.Fatalf("transport=%s probe=%s compile: %v", id, spec.ID, err)
			}
			if !profile.CandidateEligible {
				t.Fatalf("transport=%s probe=%s is not eligible: %v", id, spec.ID, profile.Reasons)
			}
		}
	}
}

func TestV2DPIPropertyVectorDistinguishesUnmeasuredAndNoEffect(t *testing.T) {
	vector := v2DPIPropertyVector{
		Version:   v2DPIPropertyProbeVersion,
		Transport: benchTransportHTTPS,
		Probes: []v2DPIPropertyObservation{
			{ID: "split", Families: []string{"split"}, State: v2DPIPropertyHelps},
			{ID: "disorder", Families: []string{"disorder"}, State: v2DPIPropertyNoEffect},
			{ID: "fake", Families: []string{"fake+split"}, State: v2DPIPropertyUnmeasured},
		},
	}
	v2FinalizeDPIPropertyVector(&vector)
	if vector.EvidenceComplete {
		t.Fatal("unmeasured probe must keep evidence incomplete")
	}
	if got := vector.FamilyScores["split"]; got != 100 {
		t.Fatalf("split score=%d want=100", got)
	}
	if got := vector.FamilyScores["disorder"]; got != -15 {
		t.Fatalf("disorder score=%d want=-15", got)
	}
	if got := vector.FamilyScores["fake+split"]; got != 0 {
		t.Fatalf("unmeasured family score=%d want=0", got)
	}
}

func TestV2DPIPropertyNormalizationRecomputesScores(t *testing.T) {
	input := &v2DPIPropertyVector{
		Version:           v2DPIPropertyProbeVersion,
		Transport:         benchTransportHTTPS,
		FamilyScores:      map[string]int{"disorder": 9999},
		PreferredFamilies: []string{"disorder"},
		Probes: []v2DPIPropertyObservation{
			{ID: "split", Families: []string{"split"}, State: v2DPIPropertyHelps},
		},
	}
	out, err := v2NormalizeDPIPropertyVector(input, benchTransportHTTPS)
	if err != nil {
		t.Fatal(err)
	}
	if out.FamilyScores["disorder"] != 0 || out.FamilyScores["split"] != 100 {
		t.Fatalf("normalization trusted caller scores: %+v", out.FamilyScores)
	}
	if len(out.PreferredFamilies) != 1 || out.PreferredFamilies[0] != "split" {
		t.Fatalf("preferred=%v want [split]", out.PreferredFamilies)
	}
}

func TestV2DPIPropertyNormalizationRejectsTransportMismatch(t *testing.T) {
	input := &v2DPIPropertyVector{Version: v2DPIPropertyProbeVersion, Transport: benchTransportQUIC}
	if _, err := v2NormalizeDPIPropertyVector(input, benchTransportHTTPS); err == nil {
		t.Fatal("transport mismatch must fail")
	}
}

func TestV2DPIPropertyProbeBudgetsAreBounded(t *testing.T) {
	cases := map[string]int{"fast": 4, "normal": 6, "thorough": 8}
	for name, want := range cases {
		mode, err := v2SelectorMode(name)
		if err != nil {
			t.Fatal(err)
		}
		if got := v2DPIPropertyProbeLimit(mode); got != want {
			t.Fatalf("mode=%s limit=%d want=%d", name, got, want)
		}
	}
}
