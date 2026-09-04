package main

import "testing"

func TestDecodeIPv4RouteHex(t *testing.T) {
	got, ok := decodeIPv4RouteHex("0101A8C0")
	if !ok || got != "192.168.1.1" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
	got, ok = decodeIPv4RouteHex("00000000")
	if !ok || got != "0.0.0.0" {
		t.Fatalf("default got %q ok=%v", got, ok)
	}
}

func TestParseKeeneticInterfaces(t *testing.T) {
	input := `
               id: GigabitEthernet1
   interface-name: ISP
             type: GigabitEthernet
      description: MGTS/MTS
             link: up
        connected: yes
            state: up
          address: 95.165.69.93
               id: WifiMaster0
   interface-name: WifiMaster0
             type: WifiMaster
      description:
             link: up
        connected: yes
            state: up
               id: Wireguard1
   interface-name: Wireguard1
             type: Wireguard
      description: DSUltra
             link: up
        connected: yes
            state: up
          address: 10.8.1.2
               id: Wireguard4
   interface-name: Wireguard4
             type: Wireguard
      description: Disabled
             link: down
        connected: no
            state: down
`
	items := parseKeeneticInterfaces(input)
	if len(items) != 4 {
		t.Fatalf("got %d interfaces, want 4: %#v", len(items), items)
	}
	if items[0].ID != "GigabitEthernet1" || items[0].Description != "MGTS/MTS" || items[0].Address != "95.165.69.93" {
		t.Fatalf("unexpected WAN parse: %#v", items[0])
	}
	if !shouldExposeKeeneticInterface(items[0]) {
		t.Fatal("active WAN with an address must be visible")
	}
	if shouldExposeKeeneticInterface(items[1]) {
		t.Fatal("WifiMaster is an infrastructure interface and must stay hidden")
	}
	if !shouldExposeKeeneticInterface(items[2]) {
		t.Fatal("active WireGuard must be visible")
	}
	if shouldExposeKeeneticInterface(items[3]) {
		t.Fatal("disabled WireGuard must be hidden")
	}
}

func TestPreferredKeeneticDisplayName(t *testing.T) {
	cases := []struct {
		item keeneticInterface
		want string
	}{
		{keeneticInterface{ID: "GigabitEthernet1", InterfaceName: "ISP", Description: "MGTS/MTS"}, "MGTS/MTS"},
		{keeneticInterface{ID: "Bridge0", InterfaceName: "Home", Description: "Home network"}, "Home network"},
		{keeneticInterface{ID: "Wireguard1", InterfaceName: "Wireguard1", Description: "DSUltra"}, "DSUltra"},
		{keeneticInterface{ID: "Something0", InterfaceName: "Friendly"}, "Friendly"},
		{keeneticInterface{ID: "Something0", InterfaceName: "1"}, "Something0"},
	}
	for _, tc := range cases {
		if got := preferredKeeneticDisplayName(tc.item); got != tc.want {
			t.Fatalf("preferredKeeneticDisplayName(%#v)=%q want %q", tc.item, got, tc.want)
		}
	}
}

func TestBestInterfaceAddress(t *testing.T) {
	got := bestInterfaceAddress([]string{"fe80::1/64", "10.8.1.2/32"})
	if got != "10.8.1.2" {
		t.Fatalf("got %q", got)
	}
}
