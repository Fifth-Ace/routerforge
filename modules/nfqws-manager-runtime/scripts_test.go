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
