package main

import "testing"

func TestSafeScriptName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"zapret-lib.lua", true},
		{"zapret_auto.LUA", true},
		{"a-b_c.lua", true},
		{"../evil.lua", false},
		{"sub/evil.lua", false},
		{".hidden.lua", false},
		{"zapret-lib.lua.old", false},
		{"not-lua.txt", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := safeScriptName(tt.name); got != tt.want {
			t.Fatalf("safeScriptName(%q)=%v want %v", tt.name, got, tt.want)
		}
	}
}

func TestScriptTargetAllowed(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/etc/nfqws2/lua/zapret-lib.lua", true},
		{"/opt/share/zapret2/lua/zapret-lib.lua", true},
		{"/opt/usr/share/nfqws2/zapret-auto.LUA", true},
		{"/opt/etc/nfqws2/lua/not-lua.txt", false},
		{"/etc/passwd.lua", false},
		{"/tmp/zapret-lib.lua", false},
		{"/opt/../etc/passwd.lua", false},
	}
	for _, tt := range tests {
		if got := scriptTargetAllowed(tt.path); got != tt.want {
			t.Fatalf("scriptTargetAllowed(%q)=%v want %v", tt.path, got, tt.want)
		}
	}
}
