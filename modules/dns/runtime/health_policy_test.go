package main

import "testing"

func TestShouldHealthCheckPolicyWithoutDefaultRoute(t *testing.T) {
	if !shouldHealthCheck(UpstreamView{UpstreamMeta: UpstreamMeta{Profile: "System"}}) {
		t.Fatal("System must be health-checked")
	}
	if shouldHealthCheck(UpstreamView{UpstreamMeta: UpstreamMeta{Profile: "Policy0", PolicyMark: 0xffffaaa, PolicyHasDefault: false}}) {
		t.Fatal("policy without default route must be skipped")
	}
	if !shouldHealthCheck(UpstreamView{UpstreamMeta: UpstreamMeta{Profile: "Policy1", PolicyMark: 0xffffaab, PolicyHasDefault: true}}) {
		t.Fatal("policy with default route must be health-checked")
	}
}
