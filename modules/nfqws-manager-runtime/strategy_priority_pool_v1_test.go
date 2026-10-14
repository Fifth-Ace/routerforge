package main

import "testing"

func nf3aTestArgs(method string) []string {
	return []string{"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=" + method}
}

func TestNF3APriorityAndDedup(t *testing.T) {
	manual := []v2SelectorCandidateInput{{ID: "my-one", Source: "custom", Args: nf3aTestArgs("multisplit:pos=1")}}
	memory := []v2CandidatePoolItem{
		{ID: "old-duplicate", MemoryClass: "WORKING", Fingerprint: v2CandidateTechniqueFingerprint(nf3aTestArgs("multisplit:pos=1")), Args: nf3aTestArgs("multisplit:pos=1")},
		{ID: "recent-working", MemoryClass: "WORKING", Fingerprint: v2CandidateTechniqueFingerprint(nf3aTestArgs("fake:blob=0x00")), Args: nf3aTestArgs("fake:blob=0x00")},
	}
	saved := []v2StoredStrategy{
		{ID: "saved", Source: "import", Args: nf3aTestArgs("multidisorder:pos=1")},
		{ID: "saved-duplicate", Source: "custom", Args: nf3aTestArgs("fake:blob=0x00")},
	}
	corpus := []v2StaticCorpusEntry{
		{ID: "corpus1", Source: "curated", Args: nf3aTestArgs("multisplit:pos=2")},
		{ID: "corpus-dup", Source: "curated", Args: nf3aTestArgs("multisplit:pos=1")},
	}
	out, meta := v2BuildPriorityCandidatePool(manual, memory, saved, corpus, 2)
	expected := []string{"my-one", "recent-working", "saved", "corpus1"}
	if len(out) != len(expected) {
		t.Fatalf("got %d candidates: %+v", len(out), out)
	}
	for i, want := range expected {
		if out[i].ID != want {
			t.Fatalf("candidate %d: %s want %s", i, out[i].ID, want)
		}
	}
	if meta.MemoryCandidates != 1 || meta.LibraryCandidates != 1 || meta.BuiltinCandidates != 1 || meta.Added != 3 {
		t.Fatalf("incorrect source totals: %+v", meta)
	}
	if len(meta.Sources) != 4 {
		t.Fatalf("sources: %v", meta.Sources)
	}
}

func TestNF3AFastCorpusLimitAndExplicitPriority(t *testing.T) {
	corpus := []v2StaticCorpusEntry{
		{ID: "one", Source: "curated", Args: nf3aTestArgs("multisplit:pos=1")},
		{ID: "two", Source: "curated", Args: nf3aTestArgs("multisplit:pos=2")},
		{ID: "three", Source: "curated", Args: nf3aTestArgs("multisplit:pos=3")},
	}
	explicit := []v2SelectorCandidateInput{{ID: "my", Source: "custom", Args: nf3aTestArgs("fake:blob=0x1234")}}
	out, meta := v2BuildPriorityCandidatePool(explicit, nil, nil, corpus, 1)
	if len(out) != 2 || out[0].ID != "my" || out[1].ID != "one" || meta.BuiltinCandidates != 1 {
		t.Fatalf("fast ordered candidates: %+v meta=%+v", out, meta)
	}
}

func TestNF3AIgnoresCorruptMemoryFingerprint(t *testing.T) {
	bad := []v2CandidatePoolItem{{ID: "bad-memory", MemoryClass: "WORKING", Fingerprint: "not-the-real-sha", Args: nf3aTestArgs("multisplit:pos=1")}}
	out, meta := v2BuildPriorityCandidatePool(nil, bad, nil, nil, 0)
	if len(out) != 0 || meta.MemoryCandidates != 0 {
		t.Fatalf("invalid memory entered queue: %+v", out)
	}
}

func TestNF3AUnstableEvidenceStaysOutOfPriorityQueue(t *testing.T) {
	args := nf3aTestArgs("multisplit:pos=3")
	memory := []v2CandidatePoolItem{{ID: "unstable", MemoryClass: "UNSTABLE", Fingerprint: v2CandidateTechniqueFingerprint(args), Args: args}}
	got, meta := v2BuildPriorityCandidatePool(nil, memory, nil, nil, 0)
	if len(got) != 0 || meta.MemoryCandidates != 0 {
		t.Fatalf("unstable memory must not be promoted: %+v", got)
	}
}
