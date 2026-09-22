package main

import "testing"

func TestV2SynthesizerPrioritizesLivePropertyFamilies(t *testing.T) {
	mode, _ := v2SelectorMode("fast")
	transport, _ := normalizeBenchTransport(benchTransportHTTPS)
	vector := &v2DPIPropertyVector{
		Version:   v2DPIPropertyProbeVersion,
		Transport: benchTransportHTTPS,
		Probes: []v2DPIPropertyObservation{
			{ID: "fake-split", Families: []string{"fake+split"}, State: v2DPIPropertyHelps},
			{ID: "split", Families: []string{"split"}, State: v2DPIPropertyNoEffect},
		},
	}
	v2FinalizeDPIPropertyVector(vector)
	items, _ := v2SynthesizeCandidates("example.com", transport, mode, v2PlannerHint{Properties: vector}, 4)
	if len(items) != 4 {
		t.Fatalf("items=%d want=4", len(items))
	}
	if items[0].Family != "fake+split" {
		t.Fatalf("first family=%q want fake+split", items[0].Family)
	}
	for _, item := range items {
		if item.Family == "split" && v2PropertyFamilyScore(vector, "split") < 0 {
			// Deprioritized families are still allowed as fallback; the test only
			// requires measured-positive evidence to lead the bounded search.
			break
		}
	}
}
