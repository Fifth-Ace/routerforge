package main

import (
	"testing"
	"time"
)

func TestNormalizeTemperature(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
		ok   bool
	}{
		{76000, 76, true},
		{55000, 55, true},
		{42.5, 42.5, true},
		{250000, 0, false},
	}
	for _, tc := range cases {
		got, ok := normalizeTemperature(tc.in)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("normalizeTemperature(%v)=(%v,%v) want (%v,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestThermalCategory(t *testing.T) {
	cases := map[string]string{
		"cpu-thermal":      "soc",
		"mcusys_thermal0":  "soc",
		"tops_thermal1":    "soc",
		"eth2p5g_thermal0": "ethernet",
		"ethsys_thermal1":  "ethernet",
		"phy0 temp":        "wifi",
		"nvme0":            "storage",
		"pmic":             "board",
		"mystery":          "other",
	}
	for input, want := range cases {
		if got := thermalCategory(input, ""); got != want {
			t.Fatalf("thermalCategory(%q)=%q want %q", input, got, want)
		}
	}
}

func TestThermalRoleMT7988(t *testing.T) {
	cases := []struct {
		name     string
		category string
		role     string
		index    int
		detail   string
	}{
		{"soc_thermal", "soc", "soc", 0, ""},
		{"mcusys_thermal0", "soc", "cpu", 0, "0-1"},
		{"mcusys_thermal1", "soc", "cpu", 1, "2-3"},
		{"eth2p5g_thermal0", "ethernet", "ethernet_2_5g", 0, ""},
		{"eth2p5g_thermal1", "ethernet", "ethernet_2_5g", 1, ""},
		{"ethsys_thermal0", "ethernet", "ethernet", 0, ""},
		{"tops_thermal1", "soc", "tops", 1, ""},
	}
	for _, tc := range cases {
		role, index, detail := thermalRole(tc.name, tc.category)
		if role != tc.role || index != tc.index || detail != tc.detail {
			t.Fatalf("thermalRole(%q)=(%q,%d,%q), want (%q,%d,%q)", tc.name, role, index, detail, tc.role, tc.index, tc.detail)
		}
	}
}

func TestMirroredHwmonSensor(t *testing.T) {
	zones := map[string]bool{
		thermalNameKey("soc_thermal"): true,
	}
	if !isMirroredHwmonSensor("soc_thermal", "", zones) {
		t.Fatal("unlabelled hwmon sensor with the same thermal-zone name must be treated as a mirror")
	}
	if isMirroredHwmonSensor("soc_thermal", "package", zones) {
		t.Fatal("labelled hwmon sensor must stay visible")
	}
	if isMirroredHwmonSensor("another_sensor", "", zones) {
		t.Fatal("different hwmon sensor must stay visible")
	}
}

func TestParseSmartctlTemperature(t *testing.T) {
	ata := "194 Temperature_Celsius     0x0022   114   099   000    Old_age   Always       -       36"
	if got, ok := parseSmartctlTemperature(ata); !ok || got != 36 {
		t.Fatalf("ATA parse=(%v,%v)", got, ok)
	}
	nvme := "Temperature:                        41 Celsius"
	if got, ok := parseSmartctlTemperature(nvme); !ok || got != 41 {
		t.Fatalf("NVMe parse=(%v,%v)", got, ok)
	}
}

func TestParseNDMCThermalsKeeneticWifiMasters(t *testing.T) {
	now := time.Date(2026, 9, 16, 9, 58, 31, 0, time.UTC)
	input := `
Interface, name = "WifiMaster0"
               id: WifiMaster0
   interface-name: WifiMaster0
             type: WifiMaster
           traits: WifiRadio
           traits: WifiMaster
           traits: MtkWifiMaster
             link: up
        connected: yes
      temperature: 52

Interface, name = "WifiMaster0/AccessPoint0"
               id: WifiMaster0/AccessPoint0
             type: AccessPoint
           traits: Wifi
           traits: AccessPoint
      temperature: 88

Interface, name = "WifiMaster1"
               id: WifiMaster1
   interface-name: WifiMaster1
             type: WifiMaster
           traits: WifiRadio
           traits: MtkWifiMaster
      temperature: 49.5
`

	got := parseNDMCThermals(input, now)
	if len(got) != 2 {
		t.Fatalf("got %d sensors, want 2: %#v", len(got), got)
	}

	if got[0].ID != "ndmc:wifimaster0" ||
		got[0].Name != "Wi-Fi · WifiMaster0" ||
		got[0].Category != "wifi" ||
		got[0].Role != "wifi" ||
		got[0].SensorIndex != 0 ||
		got[0].TempC != 52 ||
		got[0].Source != "ndmc:show interface:WifiMaster0" {
		t.Fatalf("unexpected WifiMaster0 sensor: %#v", got[0])
	}

	if got[1].ID != "ndmc:wifimaster1" ||
		got[1].SensorIndex != 1 ||
		got[1].TempC != 49.5 {
		t.Fatalf("unexpected WifiMaster1 sensor: %#v", got[1])
	}
}

func TestParseNDMCThermalsRejectsNonMasterAndInvalidTemperature(t *testing.T) {
	input := `
Interface, name = "AccessPoint"
               id: AccessPoint
             type: AccessPoint
           traits: Wifi
      temperature: 44

Interface, name = "WifiMaster0"
               id: WifiMaster0
             type: WifiMaster
      temperature: 250000
`

	got := parseNDMCThermals(input, time.Unix(0, 0))
	if len(got) != 0 {
		t.Fatalf("got unexpected sensors: %#v", got)
	}
}

func TestParseNDMCThermalsRealHopperKN3811(t *testing.T) {
	now := time.Date(2026, 9, 16, 10, 46, 35, 0, time.UTC)
	input := `
Interface, name = "WifiMaster0"
               id: WifiMaster0
   interface-name: WifiMaster0
             type: WifiMaster
           traits: WifiMaster
           traits: MtkWifiMaster
      temperature: 53
               id: WifiMaster0/AccessPoint0
   interface-name: AccessPoint
             type: AccessPoint

Interface, name = "WifiMaster1"
               id: WifiMaster1
   interface-name: WifiMaster1
             type: WifiMaster
           traits: WifiMaster
           traits: MtkWifiMaster
      temperature: 53
               id: WifiMaster1/AccessPoint0
   interface-name: AccessPoint_5G
             type: AccessPoint
`

	got := parseNDMCThermals(input, now)
	if len(got) != 2 {
		t.Fatalf("got %d sensors, want 2: %#v", len(got), got)
	}
	if got[0].ID != "ndmc:wifimaster0" || got[0].TempC != 53 {
		t.Fatalf("unexpected WifiMaster0 sensor: %#v", got[0])
	}
	if got[1].ID != "ndmc:wifimaster1" || got[1].TempC != 53 {
		t.Fatalf("unexpected WifiMaster1 sensor: %#v", got[1])
	}
}
