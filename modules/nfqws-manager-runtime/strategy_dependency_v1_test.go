package main

import "testing"

func TestP25BExternalDependencyClassifierRecognizesZ2KArtifacts(t *testing.T) {
	args := []string{
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=circular:fails=3:time=60:key=rkn_tcp:nld=2",
		"--lua-desync=tls_client_hello_clone:payload=tls_client_hello:dir=out:blob=z2k_real_www_google_com:sni_del:fallback=tls_clienthello_www_google_com",
		"--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1:seqovl=681:seqovl_pattern=tls_clienthello_www_google_com",
		"--lua-desync=fakeddisorder:payload=tls_client_hello:dir=out:pattern=tls_clienthello_vk_com",
	}
	deps := v2StrategyExternalDependencies(args)
	want := map[string]bool{
		"runtime/circular":                    false,
		"feature/tls_client_hello_clone":      false,
		"blob/z2k_real_www_google_com":        false,
		"pattern/tls_clienthello_www_google_com": false,
		"pattern/tls_clienthello_vk_com":      false,
	}
	for _, dep := range deps {
		key := dep.Kind + "/" + dep.Token
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("dependency %s missing from %#v", key, deps)
		}
	}
}

func TestP25BPortableInlineTokensHaveNoExternalDependencies(t *testing.T) {
	args := []string{
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=fake:payload=tls_client_hello:dir=out:blob=tls_clienthello:tls_mod=rnd,dupsid,sni=www.google.com",
		"--lua-desync=multisplit:payload=tls_client_hello:dir=out:pos=1:seqovl=1:seqovl_pattern=tls_clienthello",
		"--lua-desync=send:payload=empty:dir=out:repeats=2",
	}
	if deps := v2StrategyExternalDependencies(args); len(deps) != 0 {
		t.Fatalf("portable inline strategy unexpectedly has dependencies: %#v", deps)
	}
	caps := v2RegistryCapabilities(args)
	if !caps.CandidateReady {
		t.Fatalf("portable strategy should remain candidate-ready: %#v", caps)
	}
	if len(caps.ExternalDependencies) != 0 {
		t.Fatalf("portable strategy should not expose external dependencies: %#v", caps.ExternalDependencies)
	}
}

func TestP25BExternalDependencyBlocksRegistryCandidateReady(t *testing.T) {
	args := []string{
		"--filter-tcp=443",
		"--filter-l7=tls",
		"--payload=tls_client_hello",
		"--lua-desync=fake:payload=tls_client_hello:dir=out:blob=z2k_real_www_google_com",
	}
	caps := v2RegistryCapabilities(args)
	if caps.CandidateReady {
		t.Fatalf("external-dependent strategy must not be candidate-ready: %#v", caps)
	}
	if len(caps.BenchTransports) != 0 {
		t.Fatalf("external-dependent strategy must not advertise bench transports: %#v", caps.BenchTransports)
	}
	if len(caps.ExternalDependencies) == 0 {
		t.Fatalf("external-dependent strategy should expose dependencies")
	}
}

func TestP25BCurrentStaticCorpusRemainsPortable(t *testing.T) {
	for _, item := range v2StaticStrategyCorpus() {
		if deps := v2StrategyExternalDependencies(item.Args); len(deps) != 0 {
			t.Fatalf("%s leaked external dependencies after P25A: %#v", item.ID, deps)
		}
	}
}
