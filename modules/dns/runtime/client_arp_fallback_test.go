package main

import (
	"errors"
	"os/exec"
	"testing"
)

const keeneticARPFixture = `================================================================================
Name                IP                MAC                 Interface
================================================================================
Hanzo               192.168.1.10     aa:bb:cc:00:00:01   Home
                    192.168.1.11     aa:bb:cc:00:00:02   Home
HONOR-Magic7-Pro    192.168.1.12     aa:bb:cc:00:00:03   Home
Р’РµРЅС‚РёР»СЏС‚РѕСЂ С…РѕР»      192.168.1.13     aa:bb:cc:00:00:04   Home
bad-ip              garbage          aa:bb:cc:00:00:05   Home
BadMAC              192.168.1.15     invalid             Home
Public              8.8.8.8          aa:bb:cc:00:00:06   Home
`

func TestP68KeeneticARPParser(t *testing.T) {
	got := parseKeeneticARP(keeneticARPFixture)
	if len(got) != 4 {
		t.Fatalf("clients=%d want=4: %#v", len(got), got)
	}
	var named, unnamed bool
	for _, c := range got {
		if c.IP == "192.168.1.13" {
			named = c.Name == "Р’РµРЅС‚РёР»СЏС‚РѕСЂ С…РѕР»" && c.Network == "Home" && c.Access == "Network В· Home" && c.Policy == "System" && c.SSID == "" && !c.Mesh
		}
		if c.IP == "192.168.1.11" {
			unnamed = c.Name == "" && c.MAC == "aa:bb:cc:00:00:02"
		}
	}
	if !named || !unnamed {
		t.Fatalf("named=%t unnamed=%t got=%#v", named, unnamed, got)
	}
}

func TestP68FallbackOnlyOnUnsupportedHotspot(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 127")
	exitErr := cmd.Run()
	if exitErr == nil {
		t.Fatal("expected exit 127")
	}
	calls := 0
	clients, err := discoverClientsFromSources(
		func() (string, error) { return "", exitErr },
		func() (string, error) { calls++; return keeneticARPFixture, nil },
		func() map[string]string { return nil },
	)
	if err != nil || len(clients) != 4 || calls != 1 {
		t.Fatalf("fallback clients=%d calls=%d err=%v", len(clients), calls, err)
	}
	calls = 0
	_, err = discoverClientsFromSources(
		func() (string, error) { return "", errors.New("timeout") },
		func() (string, error) { calls++; return keeneticARPFixture, nil },
		func() map[string]string { return nil },
	)
	if err == nil || calls != 0 {
		t.Fatalf("masked transient error: calls=%d err=%v", calls, err)
	}
}

func TestP68HotspotPreferred(t *testing.T) {
	calls := 0
	clients, err := discoverClientsFromSources(
		func() (string, error) {
			return "host:\n  ip: 192.168.1.20\n  mac: aa:bb:cc:00:00:20\n  name: MyPC\n", nil
		},
		func() (string, error) { calls++; return keeneticARPFixture, nil },
		func() map[string]string { return nil },
	)
	if err != nil || calls != 0 || len(clients) != 1 || clients[0].Name != "MyPC" {
		t.Fatalf("hotspot regression: %#v calls=%d err=%v", clients, calls, err)
	}
}

func TestP68ARPFailureNotHidden(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 127")
	err127 := cmd.Run()
	_, err := discoverClientsFromSources(
		func() (string, error) { return "", err127 },
		func() (string, error) { return "", errors.New("arp failed") },
		func() map[string]string { return map[string]string{"192.168.1.44": "aa:bb:cc:00:00:44"} },
	)
	if err == nil || err.Error() != "arp failed" {
		t.Fatalf("arp error hidden: %v", err)
	}
}
