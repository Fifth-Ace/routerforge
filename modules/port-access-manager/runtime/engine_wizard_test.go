package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func exampleWizard() EngineWizardRequest {
	return EngineWizardRequest{Engine: "knockd", Interface: "eth3", Knock: []int{7777, 8888, 6666}, SequenceSeconds: 15, AccessSeconds: 120, Services: []ProtectedService{{Name: "web", Address: "192.168.10.103", Port: 18080}, {Name: "ssh", Address: "192.168.10.104", Port: 22}}}
}
func TestWizardValidation(t *testing.T) {
	valid := exampleWizard()
	if err := validateWizard(valid); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*EngineWizardRequest)
	}{
		{"invalid interface", func(r *EngineWizardRequest) { r.Interface = "eth3;rm -rf /" }},
		{"repeated knock", func(r *EngineWizardRequest) { r.Knock[2] = r.Knock[1] }},
		{"service port overlap", func(r *EngineWizardRequest) { r.Services[0].Port = 7777 }},
		{"duplicate service", func(r *EngineWizardRequest) { r.Services = append(r.Services, r.Services[0]) }},
		{"invalid label", func(r *EngineWizardRequest) { r.Services[0].Name = "web;reboot" }},
		{"invalid IP", func(r *EngineWizardRequest) { r.Services[0].Address = "127.0.0.1" }},
		{"no services", func(r *EngineWizardRequest) { r.Services = nil }},
		{"lifetime", func(r *EngineWizardRequest) { r.AccessSeconds = 0 }},
		{"wrong mode", func(r *EngineWizardRequest) { r.Engine = "odin" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := exampleWizard()
			tc.change(&r)
			if err := validateWizard(r); err == nil {
				t.Fatal("accepted unsafe input")
			}
		})
	}
	valid.Engine = "fwknopd"
	valid.Knock = nil
	valid.Interface = "eth3"
	if err := validateWizard(valid); err != nil {
		t.Fatal(err)
	}
}
func TestWizardDescribe(t *testing.T) {
	d, err := describeWizard(exampleWizard())
	if err != nil || !strings.Contains(d, "192.168.10.104:22") {
		t.Fatalf("%q %v", d, err)
	}
}
func TestSPAKeys(t *testing.T) {
	a, b, err := generateSPAKeys()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("identical keys")
	}
	for _, v := range []string{a, b} {
		decoded, e := base64.StdEncoding.DecodeString(v)
		if e != nil || len(decoded) != 32 {
			t.Fatalf("bad key: %v", e)
		}
	}
}
