package main

import "testing"

func TestModuleTargetPathPreservesTrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{name: "health", in: "", want: "/v1/health", ok: true},
		{name: "api leaf", in: "info", want: "/v1/info", ok: true},
		{name: "ui index", in: "ui/index.html", want: "/v1/ui/index.html", ok: true},
		{name: "ui directory", in: "ui/", want: "/v1/ui/", ok: true},
		{name: "nested asset directory", in: "ui/assets/", want: "/v1/ui/assets/", ok: true},
		{name: "reject traversal", in: "ui/../secret", want: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := moduleTargetPath(tt.in)
			if ok != tt.ok {
				t.Fatalf("ok=%v, want %v (path=%q)", ok, tt.ok, got)
			}
			if got != tt.want {
				t.Fatalf("path=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestModuleTargetPathUIRedirectRegression(t *testing.T) {
	got, ok := moduleTargetPath("ui/")
	if !ok {
		t.Fatal("ui directory rejected")
	}
	if got == "/v1/ui" {
		t.Fatal("trailing slash was lost; this reintroduces the nested Core shell/404 redirect bug")
	}
}
