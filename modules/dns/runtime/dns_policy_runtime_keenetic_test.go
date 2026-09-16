package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestDNSPolicyKeeneticPrimitiveSnapshotUsesOnlyProvenNarrowReads(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rci/ip/policy":
			_, _ = w.Write([]byte(`{"Policy0":{"description":"123"}}`))
		case "/rci/ip/hotspot/host":
			_, _ = w.Write([]byte(`[{"mac":"b","policy":"Policy1"},{"mac":"a"}]`))
		case "/rci/show/ip/policy":
			_, _ = w.Write([]byte(`{"Policy0":{"mark":0},"Policy1":{"mark":1}}`))
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	primitive := newDNSPolicyKeeneticPrimitive(newDNSRCIClient(server.URL+"/rci"), func() error { return nil })
	snapshot, err := primitive.SnapshotRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Identity == "" || snapshot.RuleCount != 0 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	want := []string{"/rci/ip/policy", "/rci/ip/hotspot/host", "/rci/show/ip/policy"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestDNSPolicyKeeneticPrimitiveIdentityIgnoresHostArrayOrder(t *testing.T) {
	hostBodies := []string{
		`[{"mac":"b","policy":"Policy1"},{"mac":"a"}]`,
		`[{"mac":"a"},{"mac":"b","policy":"Policy1"}]`,
	}
	hostRead := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rci/ip/policy":
			_, _ = w.Write([]byte(`{"Policy1":{"description":"nfqws"},"Policy0":{"description":"123"}}`))
		case "/rci/ip/hotspot/host":
			body := hostBodies[hostRead%len(hostBodies)]
			hostRead++
			_, _ = w.Write([]byte(body))
		case "/rci/show/ip/policy":
			_, _ = w.Write([]byte(`{"Policy1":{"mark":1},"Policy0":{"mark":0}}`))
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	primitive := newDNSPolicyKeeneticPrimitive(newDNSRCIClient(server.URL+"/rci"), func() error { return nil })
	first, err := primitive.SnapshotRuntime()
	if err != nil {
		t.Fatal(err)
	}
	second, err := primitive.SnapshotRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity != second.Identity {
		t.Fatalf("identity changed with host order: %s != %s", first.Identity, second.Identity)
	}
}

func TestDNSPolicyKeeneticPrimitiveVerifySnapshotDetectsChange(t *testing.T) {
	description := "123"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rci/ip/policy":
			_, _ = w.Write([]byte(`{"Policy0":{"description":"` + description + `"}}`))
		case "/rci/ip/hotspot/host":
			_, _ = w.Write([]byte(`[]`))
		case "/rci/show/ip/policy":
			_, _ = w.Write([]byte(`{"Policy0":{"mark":0}}`))
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	primitive := newDNSPolicyKeeneticPrimitive(newDNSRCIClient(server.URL+"/rci"), func() error { return nil })
	snapshot, err := primitive.SnapshotRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if err := primitive.VerifyRuntimeSnapshot(snapshot); err != nil {
		t.Fatalf("unchanged snapshot failed verification: %v", err)
	}
	description = "changed"
	if err := primitive.VerifyRuntimeSnapshot(snapshot); err == nil {
		t.Fatal("changed runtime identity must fail verification")
	}
}

func TestDNSPolicyKeeneticPrimitiveMutationFailsClosed(t *testing.T) {
	primitive := newDNSPolicyKeeneticPrimitive(nil, func() error { return nil })
	for name, err := range map[string]error{
		"apply":    primitive.ApplyCanonicalRules(nil),
		"verify":   primitive.VerifyCanonicalRules(nil),
		"rollback": primitive.RestoreRuntime(DNSPolicyRuntimeSnapshot{}),
	} {
		if !errors.Is(err, errDNSPolicyDataplaneMappingUnknown) {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}
