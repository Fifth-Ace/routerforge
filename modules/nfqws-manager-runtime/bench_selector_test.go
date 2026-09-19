package main

import "testing"

func TestBenchSelectorContractIsNarrowAndLocked(t *testing.T) {
	got := buildBenchSelectorContract()

	if !got.Implemented || !got.MutationImplemented {
		t.Fatalf("selector mutation contract must be active for controlled TLS smoke: %+v", got)
	}
	if !got.IPv4Only || got.Protocol != "tcp" || got.RemotePort != 443 {
		t.Fatalf("unexpected first active selector scope=%+v", got)
	}
	if !got.DestinationExact || !got.LocalPortExact || !got.SingleSessionOnly {
		t.Fatalf("selector must use exact tuple and one session: %+v", got)
	}
	if got.PacketMark != 0x40000000 || got.PacketMarkMask != 0x40000000 {
		t.Fatalf("unexpected packet mark=%#x mask=%#x", got.PacketMark, got.PacketMarkMask)
	}
	if got.ConnmarkRequired || got.OwnerMatchRequired || got.RawTableRequired {
		t.Fatalf("selector must not depend on connmark/owner/raw: %+v", got)
	}
	if !got.ProductionQueueBypass || !got.MarkClearedAfterHook {
		t.Fatalf("selector bypass/mark cleanup contract missing: %+v", got)
	}
}

func TestBenchSelectorHooksBracketProductionNFQWS(t *testing.T) {
	got := buildBenchSelectorContract()

	if len(got.Hooks) != 4 {
		t.Fatalf("hooks=%d want=4", len(got.Hooks))
	}

	want := []struct {
		chain    string
		relation string
		anchor   string
	}{
		{"POSTROUTING", "before", "nfqws_post"},
		{"POSTROUTING", "after", "nfqws_post"},
		{"PREROUTING", "before", "nfqws_pre"},
		{"PREROUTING", "after", "nfqws_pre"},
	}

	for i, expected := range want {
		hook := got.Hooks[i]
		if hook.Table != "mangle" ||
			hook.Chain != expected.chain ||
			hook.Relation != expected.relation ||
			hook.Anchor != expected.anchor {
			t.Fatalf("hook[%d]=%+v want=%+v", i, hook, expected)
		}
	}
}
