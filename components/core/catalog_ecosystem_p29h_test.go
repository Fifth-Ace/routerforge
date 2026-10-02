package main

import "testing"

func TestArchitectureAwareFeedContent(t *testing.T) {
	output := "arch all 1\narch aarch64-3.10 10\narch mipsel-3.4 5\n"
	got, err := architectureAwareFeedContent(
		output,
		"src/gz feedly_{arch} https://spatiumstas.github.io/feedly/{arch}",
		[]string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "src/gz feedly_aarch64-3.10 https://spatiumstas.github.io/feedly/aarch64-3.10\n" +
		"src/gz feedly_mipsel-3.4 https://spatiumstas.github.io/feedly/mipsel-3.4"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestArchitectureAwareFeedRejectsUnsupportedRouter(t *testing.T) {
	_, err := architectureAwareFeedContent(
		"arch all 1\narch x86_64-3.2 10\n",
		"src/gz feedly_{arch} https://spatiumstas.github.io/feedly/{arch}",
		[]string{"aarch64-3.10", "armv7-3.2", "mips-3.4", "mipsel-3.4"},
	)
	if err == nil {
		t.Fatal("unsupported opkg architecture accepted")
	}
}

func TestTGWSProxyGoStructuredLifecycle(t *testing.T) {
	item := tgWSProxyGoCatalogItem()
	if item.Install.PreviewOnly {
		t.Fatal("install remained preview-only")
	}
	if item.Install.Method != "structured" || !executableCatalogPlan(item.Install) {
		t.Fatalf("install=%#v", item.Install)
	}
	if err := validateCatalogPlan(item.Install); err != nil {
		t.Fatalf("install plan invalid: %v", err)
	}
	if item.Update.Method != "structured" || !executableCatalogPlan(item.Update) {
		t.Fatalf("update=%#v", item.Update)
	}
	if err := validateCatalogPlan(item.Update); err != nil {
		t.Fatalf("update plan invalid: %v", err)
	}

	foundArchFeed := false
	for _, step := range item.Install.Steps {
		if step.Type == "write-opkg-feed-arch" {
			foundArchFeed = true
			if step.Path != "/opt/etc/opkg/feedly.conf" {
				t.Fatalf("feed path=%q", step.Path)
			}
			if len(step.Args) != 4 {
				t.Fatalf("supported architectures=%v", step.Args)
			}
		}
	}
	if !foundArchFeed {
		t.Fatal("architecture-aware feed step missing")
	}
}
