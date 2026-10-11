package main

import "testing"

func TestOdinConfigurationValidation(t *testing.T) {
	s := odinSettings{WAN: "eth3", IP: "192.168.1.2", Port: 3389, Knock: [3]int{7777, 8888, 6666}, Window: 15, TTL: 28800}
	if err := s.valid(); err != nil {
		t.Fatal(err)
	}
	x := s
	x.Knock[1] = x.Port
	if x.valid() == nil {
		t.Fatal("duplicate port accepted")
	}
	x = s
	x.WAN = "eth3;touch /tmp/test"
	if x.valid() == nil {
		t.Fatal("shell injection accepted")
	}
	x = s
	x.IP = "127.0.0.1"
	if x.valid() == nil {
		t.Fatal("loopback target accepted")
	}
	x = s
	x.TTL = 0
	if x.valid() == nil {
		t.Fatal("invalid TTL accepted")
	}
}
