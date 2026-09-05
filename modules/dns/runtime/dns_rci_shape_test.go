package main

import (
	"encoding/json"
	"testing"
)

func TestDNSSavedNameServersAcceptArrayShape(t *testing.T) {
	var got dnsSavedNameServers
	if err := json.Unmarshal([]byte(`[]`), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Server) != 0 {
		t.Fatalf("server count = %d, want 0", len(got.Server))
	}

	if err := json.Unmarshal([]byte(`[{"address":"1.1.1.1","domain":"ru"}]`), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Server) != 1 || got.Server[0]["address"] != "1.1.1.1" {
		t.Fatalf("unexpected array shape decode: %#v", got.Server)
	}
}

func TestDNSSavedNameServersAcceptEnvelopeShape(t *testing.T) {
	var got dnsSavedNameServers
	if err := json.Unmarshal([]byte(`{"server":[{"address":"9.9.9.9"}]}`), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Server) != 1 || got.Server[0]["address"] != "9.9.9.9" {
		t.Fatalf("unexpected envelope shape decode: %#v", got.Server)
	}
}

func TestDNSSavedNameServersAcceptNullAsEmpty(t *testing.T) {
	var got dnsSavedNameServers
	if err := json.Unmarshal([]byte(`null`), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Server) != 0 {
		t.Fatalf("server count = %d, want 0", len(got.Server))
	}
}
