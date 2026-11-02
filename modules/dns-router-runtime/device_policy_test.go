package dnsrouter

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const macA = "02:11:22:33:44:55"
const macB = "02:11:22:33:44:66"

func deviceFixture() Config {
	c := fixture()
	c.Service = &ServiceConfig{Listen: "192.168.10.1:5353", DefaultUpstream: "one", CacheBytes: 65536, AllowedNetworks: []string{"192.168.10.0/24"}}
	c.Advanced = &AdvancedDNS{Devices: []DevicePolicy{{MAC: macA, Name: "Laptop", Target: &DeviceTarget{Upstream: "two"}}}, Groups: []DevicePolicyGroup{{ID: "office", Name: "Office", MACs: []string{macA, macB}, Target: &DeviceTarget{Upstream: "one"}}}, Rules: []DeviceDomainRule{{ID: "device-domain", MAC: macA, Domain: "api.example.com", Match: "exact", Priority: 100, Target: DeviceTarget{Upstream: "one"}}}}
	return c
}
func TestP21DevicePrecedenceAndInheritance(t *testing.T) {
	c := deviceFixture()
	cases := []struct {
		name, mac, layer, upstream string
		inherit                    bool
	}{
		{"api.example.com", macA, "device_domain", "one", false},
		{"example.com", macA, "device", "two", false},
		{"unknown.net", macA, "device", "two", false},
		{"api.example.com", macB, "device_group", "one", false},
		{"api.example.com", "", "common_domain", "two", false},
		{"unknown.net", "", "default", "one", false},
		{"example.com", macA, "common_domain", "one", true},
		{"api.example.com", macB, "common_domain", "two", true},
		{"unknown.net", macA, "device", "two", true},
		{"router.lan", macA, "local_namespace", "", false},
		{"my.keenetic.net", macA, "local_namespace", "", false},
		{"_non\\032host.example", macA, "device", "two", false},
	}
	for _, tc := range cases {
		t.Run(tc.name+tc.mac+tc.layer, func(t *testing.T) {
			c.Advanced.Devices[0].InheritDomains = tc.inherit
			c.Advanced.Groups[0].InheritDomains = tc.inherit
			e, err := Compile(c)
			if err != nil {
				t.Fatal(err)
			}
			d, err := e.EvaluateClientDNSQuestion(tc.name, tc.mac)
			if err != nil || d.Layer != tc.layer || d.Upstream != tc.upstream {
				t.Fatalf("%+v %v", d, err)
			}
			if tc.layer == "device_group" && d.DeviceGroup != "office" {
				t.Fatal(d)
			}
		})
	}
}
func TestP21DeviceValidationAndImmutableTargets(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"multicast":    func(c *Config) { c.Advanced.Devices[0].MAC = "ff:11:22:33:44:55" },
		"noncanonical": func(c *Config) { c.Advanced.Devices[0].MAC = "02:AA:22:33:44:55" },
		"duplicate":    func(c *Config) { c.Advanced.Devices = append(c.Advanced.Devices, c.Advanced.Devices[0]) },
		"two_groups": func(c *Config) {
			g := c.Advanced.Groups[0]
			g.ID = "second"
			c.Advanced.Groups = append(c.Advanced.Groups, g)
		},
		"missing_target": func(c *Config) { c.Advanced.Devices[0].Target.Upstream = "missing" },
		"native": func(c *Config) {
			c.Upstreams[1] = Upstream{ID: "two", Transport: "native", Address: "native://System/./two"}
		},
		"protected_rule": func(c *Config) { c.Advanced.Rules[0].Domain = "local" },
		"invalid_group":  func(c *Config) { c.Advanced.Devices[0].Target.GroupID = "missing" },
	} {
		t.Run(name, func(t *testing.T) {
			c := deviceFixture()
			change(&c)
			if _, err := Compile(c); err == nil {
				t.Fatal("accepted unsafe config")
			}
		})
	}
	c := deviceFixture()
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	c.Advanced.Devices[0].Target.Upstream = "one"
	c.Advanced.Groups[0].Target.Upstream = "two"
	for _, tc := range []struct{ mac, want string }{{macA, "two"}, {macB, "one"}} {
		d, _ := e.EvaluateClientDNSQuestion("outside.example", tc.mac)
		if d.Upstream != tc.want {
			t.Fatal("compiled target changed", d)
		}
	}
}
func TestP21StrictDeviceGroupChainAndNoRecursion(t *testing.T) {
	c := deviceFixture()
	c.Upstreams[0].Fallbacks = []string{"two"}
	c.Groups = []DNSGroup{{ID: "private", Name: "Private", Servers: []string{"two"}}}
	c.Advanced.Devices[0].Target = &DeviceTarget{Upstream: "two", GroupID: "private"}
	e, err := Compile(c)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := e.EvaluateClientDNSQuestion("outside.example", macA)
	chain, err := e.UpstreamChainForDecision(d.Decision)
	if err != nil || len(chain) != 1 || chain[0].ID != "two" {
		t.Fatal(chain, err)
	}
	c.Upstreams[1].Address = "192.168.10.1:53"
	if _, err = Compile(c); err == nil {
		t.Fatal("router recursion accepted")
	}
}
func TestP21LegacyRevisionAndImportPreservePolicies(t *testing.T) {
	c := fixture()
	raw, _ := json.Marshal(c)
	if bytes.Contains(raw, []byte("advanced")) {
		t.Fatal("legacy encoding changed")
	}
	before := revision(c)
	decoded, err := DecodeConfig(bytes.NewReader(raw))
	if err != nil || revision(decoded) != before {
		t.Fatal("legacy revision changed", err)
	}
	c = deviceFixture()
	c.Groups = []DNSGroup{{ID: "g", Name: "Group", Servers: []string{"one"}}}
	for _, mode := range []string{"append", "replace"} {
		p, err := PreviewImport(c, ImportRequest{Text: "new.example", Upstream: "one", Match: "suffix", Priority: 100, Mode: mode})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Config.Advanced, c.Advanced) || !reflect.DeepEqual(p.Config.Service, c.Service) || !reflect.DeepEqual(p.Config.Groups, c.Groups) {
			t.Fatal("import dropped config")
		}
	}
	e, err := Compile(fixture())
	if err != nil || e.DevicePoliciesActive() {
		t.Fatal(err)
	}
	c = deviceFixture()
	c.Advanced = &AdvancedDNS{Devices: []DevicePolicy{{MAC: macA, Name: "name only"}}}
	e, err = Compile(c)
	if err != nil || e.DevicePoliciesActive() {
		t.Fatal("metadata disabled shared cache", err)
	}
	c.Advanced.Devices[0].Name = strings.Repeat("я", 81)
	if _, err = Compile(c); err == nil {
		t.Fatal("unbounded name")
	}
}
