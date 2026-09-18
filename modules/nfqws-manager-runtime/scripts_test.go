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
		{"zapret-lib.lua.gz", false},
		{"not-lua.txt", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := safeScriptName(tt.name); got != tt.want {
			t.Fatalf("safeScriptName(%q)=%v want %v", tt.name, got, tt.want)
		}
	}
}

func TestScriptDisplayName(t *testing.T) {
	tests := []struct {
		input string
		name  string
		ok    bool
	}{
		{"zapret-lib.lua", "zapret-lib.lua", true},
		{"zapret-lib.lua.gz", "zapret-lib.lua", true},
		{"/opt/etc/nfqws2/lua/zapret-auto.LUA.GZ", "zapret-auto.LUA", true},
		{"not-lua.txt", "", false},
	}
	for _, tt := range tests {
		name, ok := scriptDisplayName(tt.input)
		if ok != tt.ok || name != tt.name {
			t.Fatalf("scriptDisplayName(%q)=(%q,%v) want (%q,%v)", tt.input, name, ok, tt.name, tt.ok)
		}
	}
}

func TestScriptTargetAllowed(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/opt/etc/nfqws2/lua/zapret-lib.lua", true},
		{"/opt/etc/nfqws2/lua/zapret-lib.lua.gz", true},
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
