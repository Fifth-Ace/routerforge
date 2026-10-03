package main

import "testing"

func TestP29TLegacyXrayAutoRemainsPreviewOnlyUntilChainIsImmutable(t *testing.T) {
	item := keeneticXrayAutoCatalogItem()

	if item.ProjectURL != "https://github.com/kuzzrus/keenetic_xray_installer" {
		t.Fatalf("canonical project URL mismatch: %s", item.ProjectURL)
	}
	if item.Publisher.Name != "kuzzrus" {
		t.Fatalf("publisher mismatch: %#v", item.Publisher)
	}
	if item.Install.Method != "manual" || !item.Install.PreviewOnly {
		t.Fatalf("legacy xray install must remain preview/manual: %#v", item.Install)
	}
	if item.Update.Method != "" || item.Remove.Method != "" {
		t.Fatalf("legacy xray lifecycle must not expose automatic update/remove: update=%#v remove=%#v", item.Update, item.Remove)
	}
	if deriveCatalogActions(item).Install || deriveCatalogActions(item).Update || deriveCatalogActions(item).Remove {
		t.Fatalf("legacy xray lifecycle unexpectedly executable: %#v", deriveCatalogActions(item))
	}
}
