package main

import (
	"reflect"
	"testing"
)

func TestKnockForwardHookDryRun(t *testing.T) {
	apply, undo, err := buildKnockHook("eth3", 2222)
	if err != nil {
		t.Fatal(err)
	}
	wantApply := [][]string{{"iptables", "-t", "filter", "-I", "FORWARD", "1", "-i", "eth3", "-p", "tcp", "--dport", "2222", "-j", "RF_PORT_KNOCK"}}
	wantUndo := [][]string{{"iptables", "-t", "filter", "-D", "FORWARD", "-i", "eth3", "-p", "tcp", "--dport", "2222", "-j", "RF_PORT_KNOCK"}}
	if !reflect.DeepEqual(apply, wantApply) || !reflect.DeepEqual(undo, wantUndo) {
		t.Fatalf("apply=%v undo=%v", apply, undo)
	}
}
func TestKnockForwardHookRejectsInvalidScope(t *testing.T) {
	for _, tc := range []struct {
		iface string
		port  int
	}{
		{"", 2222}, {"eth3 -j ACCEPT", 2222}, {"eth3/1", 2222}, {"eth3", 0}, {"eth3", 65536},
	} {
		if a, u, err := buildKnockHook(tc.iface, tc.port); err == nil || a != nil || u != nil {
			t.Fatalf("accepted %+v: %v %v", tc, a, u)
		}
	}
}
