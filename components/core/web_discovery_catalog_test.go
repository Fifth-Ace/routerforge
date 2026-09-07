package main

import "testing"

func TestRFSynthesizeRuntimeWebCatalogCreatesRuntimeItem(t *testing.T) {
	surfaces := []rfDetectedWebSurface{
		{
			Address:     "0.0.0.0",
			Port:        8765,
			Scheme:      "http",
			ProbeURL:    "http://127.0.0.1:8765/",
			Path:        "/login",
			StatusCode:  302,
			ContentType: "text/html",
			Title:       "Mystery Panel",
			Owners: []rfWebListenerOwner{
				{PID: 123, Process: "mysteryd", Package: "mystery-manager"},
			},
		},
	}

	items := rfSynthesizeRuntimeWebCatalog(nil, surfaces, map[string]string{"mystery-manager": "1.2.3"})
	if len(items) != 1 {
		t.Fatalf("items=%d %#v", len(items), items)
	}
	item := items[0]
	if item.ID == "" || item.Kind != "integration" || item.Name != "Mystery Panel" {
		t.Fatalf("identity=%#v", item)
	}
	if !item.Installed || item.State != "installed_external" || !item.ServiceRunning {
		t.Fatalf("runtime state=%#v", item)
	}
	if item.Web == nil || item.Web.Scheme != "http" || item.Web.Port != 8765 || item.Web.Path != "/login" || item.Web.Mode != "probe-required" || !item.Web.Embed {
		t.Fatalf("web=%#v", item.Web)
	}
	if item.WebProbeHost != "127.0.0.1" || item.RegistrySource != "runtime-web-discovery" || item.WebPortSource != "runtime-listener" {
		t.Fatalf("provenance=%#v", item)
	}
	if item.Trust.Status != "runtime-local" || item.Actions.Install || item.Actions.Update || item.Actions.Remove {
		t.Fatalf("trust/actions=%#v %#v", item.Trust, item.Actions)
	}
	if item.Version != "1.2.3" || len(item.Detection.Packages) != 1 || item.Detection.Packages[0] != "mystery-manager" {
		t.Fatalf("package=%#v", item)
	}
}

func TestRFSynthesizeRuntimeWebCatalogSkipsKnownIntegration(t *testing.T) {
	existing := []catalogItem{
		{
			ID:        "known",
			Installed: true,
			Detection: catalogDetection{Packages: []string{"known-manager"}},
			Web:       &catalogWebMetadata{Port: 8765},
		},
	}
	surfaces := []rfDetectedWebSurface{
		{
			Address:  "127.0.0.1",
			Port:     8765,
			Scheme:   "http",
			ProbeURL: "http://127.0.0.1:8765/",
			Owners:   []rfWebListenerOwner{{Process: "known-manager", Package: "known-manager"}},
		},
	}
	if items := rfSynthesizeRuntimeWebCatalog(existing, surfaces, nil); len(items) != 0 {
		t.Fatalf("expected no duplicate runtime item: %#v", items)
	}
}

func TestRFSynthesizeRuntimeWebCatalogSkipsRouterForgeCore(t *testing.T) {
	surfaces := []rfDetectedWebSurface{
		{
			Address:  "0.0.0.0",
			Port:     2233,
			Scheme:   "http",
			ProbeURL: "http://127.0.0.1:2233/",
			Owners:   []rfWebListenerOwner{{Process: "routerforge", Package: "routerforge-core", Executable: "/opt/bin/routerforge"}},
		},
	}
	if items := rfSynthesizeRuntimeWebCatalog(nil, surfaces, nil); len(items) != 0 {
		t.Fatalf("RouterForge must not rediscover itself: %#v", items)
	}
}

func TestRFRuntimeWebIDStableAndSafe(t *testing.T) {
	identity := "mystery-manager|http|8765"
	a := rfRuntimeWebID(identity)
	b := rfRuntimeWebID(identity)
	if a != b || !safeCatalogWebProbeID(a) {
		t.Fatalf("id=%q second=%q", a, b)
	}
}

func TestRFRuntimeWebPathFailClosed(t *testing.T) {
	for _, input := range []string{"", "login", "/ok\r\nInjected: yes"} {
		if got := rfRuntimeWebPath(input); got != "/" {
			t.Fatalf("input=%q got=%q", input, got)
		}
	}
	if got := rfRuntimeWebPath("/login"); got != "/login" {
		t.Fatalf("got=%q", got)
	}
}
