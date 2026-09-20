package main

import "testing"

func v2ImportTestArgs() []string {
	return []string{
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=multisplit:pos=1,midsld",
	}
}

func TestV2ImportStrategiesAddsAndDedupes(t *testing.T) {
	args := v2ImportTestArgs()
	doc := v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{}}
	next, result, err := v2ImportStrategies(doc, []v2StrategyImportItem{
		{Name: "Zapret profile 1", Args: args},
		{Name: "Zapret duplicate", Args: args},
	}, "2026-09-21T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 || result.Existing != 0 || len(next.Strategies) != 1 {
		t.Fatalf("result=%+v entries=%d", result, len(next.Strategies))
	}
	if next.Strategies[0].Source != "zapret" {
		t.Fatalf("source=%q", next.Strategies[0].Source)
	}
}

func TestV2ImportStrategiesKeepsExisting(t *testing.T) {
	args := v2ImportTestArgs()
	fp := v2StrategyFingerprint(args)
	existing := v2StoredStrategy{
		ID: "s-" + fp[:16], Name: "Existing", Source: "custom", Args: args,
		Fingerprint: fp, CreatedAt: "old", UpdatedAt: "old",
	}
	doc := v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{existing}}
	next, result, err := v2ImportStrategies(doc, []v2StrategyImportItem{
		{Name: "Imported duplicate", Args: args},
	}, "new")
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || result.Existing != 1 || len(next.Strategies) != 1 {
		t.Fatalf("result=%+v entries=%d", result, len(next.Strategies))
	}
	if next.Strategies[0].Name != "Existing" || next.Strategies[0].UpdatedAt != "old" {
		t.Fatalf("existing strategy was mutated: %+v", next.Strategies[0])
	}
}

func TestV2ImportStrategiesRejectsWholeBatchOnInvalidItem(t *testing.T) {
	valid := v2ImportTestArgs()
	doc := v2StrategyLibraryDocument{Version: 1, Strategies: []v2StoredStrategy{}}
	next, result, err := v2ImportStrategies(doc, []v2StrategyImportItem{
		{Name: "Valid", Args: valid},
		{Name: "", Args: valid},
	}, "now")
	if err == nil {
		t.Fatal("invalid batch accepted")
	}
	if len(next.Strategies) != 0 || result.Added != 0 {
		t.Fatalf("partial import occurred: %+v %+v", next, result)
	}
}
