package main

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	rfRuntimeWebDiscoveryTTL      = 30 * time.Second
	rfRuntimeWebDiscoveryTimeout  = 2200 * time.Millisecond
	rfRuntimeWebDiscoveryProcRoot = "/proc"
	rfRuntimeWebDiscoveryOpkgInfo = "/opt/lib/opkg/info"
)

var rfRuntimeWebCatalogState struct {
	mu      sync.Mutex
	at      time.Time
	items   []catalogItem
	lastErr string
}

func rfApplyRuntimeWebDiscovery(snapshot *catalogSnapshot, installed map[string]string) {
	if snapshot == nil {
		return
	}
	items := rfRuntimeWebCatalogSnapshot(snapshot.Integrations, installed)
	if len(items) == 0 {
		return
	}
	snapshot.Integrations = append(snapshot.Integrations, items...)
	sort.Slice(snapshot.Integrations, func(i, j int) bool {
		if snapshot.Integrations[i].Category != snapshot.Integrations[j].Category {
			return snapshot.Integrations[i].Category < snapshot.Integrations[j].Category
		}
		if snapshot.Integrations[i].Name != snapshot.Integrations[j].Name {
			return snapshot.Integrations[i].Name < snapshot.Integrations[j].Name
		}
		return snapshot.Integrations[i].ID < snapshot.Integrations[j].ID
	})
}

func rfRuntimeWebCatalogSnapshot(existing []catalogItem, installed map[string]string) []catalogItem {
	rfRuntimeWebCatalogState.mu.Lock()
	defer rfRuntimeWebCatalogState.mu.Unlock()

	now := time.Now()
	if !rfRuntimeWebCatalogState.at.IsZero() && now.Sub(rfRuntimeWebCatalogState.at) < rfRuntimeWebDiscoveryTTL {
		return rfCloneCatalogItems(rfRuntimeWebCatalogState.items)
	}

	ctx, cancel := context.WithTimeout(context.Background(), rfRuntimeWebDiscoveryTimeout)
	defer cancel()

	listeners, err := rfDiscoverLocalTCPListeners(rfRuntimeWebDiscoveryProcRoot, rfRuntimeWebDiscoveryOpkgInfo)
	if err != nil {
		rfRuntimeWebCatalogState.at = now
		rfRuntimeWebCatalogState.lastErr = err.Error()
		return rfCloneCatalogItems(rfRuntimeWebCatalogState.items)
	}

	surfaces := rfDetectLocalWebSurfaces(ctx, listeners)
	items := rfSynthesizeRuntimeWebCatalog(existing, surfaces, installed)
	rfRuntimeWebCatalogState.at = now
	rfRuntimeWebCatalogState.items = rfCloneCatalogItems(items)
	rfRuntimeWebCatalogState.lastErr = ""
	return items
}

func rfSynthesizeRuntimeWebCatalog(existing []catalogItem, surfaces []rfDetectedWebSurface, installed map[string]string) []catalogItem {
	if len(surfaces) == 0 {
		return nil
	}

	knownPackages := map[string]struct{}{}
	knownProcesses := map[string]struct{}{}
	knownPorts := map[int]struct{}{}
	for _, item := range existing {
		if !item.Installed || item.Web == nil {
			continue
		}
		for _, pkg := range item.Detection.Packages {
			pkg = strings.ToLower(strings.TrimSpace(pkg))
			if pkg != "" {
				knownPackages[pkg] = struct{}{}
			}
		}
		for _, process := range item.ProcessNames {
			process = strings.ToLower(strings.TrimSpace(process))
			if process != "" {
				knownProcesses[process] = struct{}{}
			}
		}
		if item.Web.Port > 0 {
			knownPorts[item.Web.Port] = struct{}{}
		}
	}

	items := make([]catalogItem, 0, len(surfaces))
	seen := map[string]struct{}{}
	for _, surface := range surfaces {
		owner := rfPreferredRuntimeWebOwner(surface.Owners)
		if rfRuntimeWebSelfSurface(surface, owner) {
			continue
		}
		if rfRuntimeWebMatchesKnown(owner, surface.Port, knownPackages, knownProcesses, knownPorts) {
			continue
		}

		probeHost := rfRuntimeWebProbeHost(surface)
		if probeHost == "" || surface.Port < 1 || surface.Port > 65535 {
			continue
		}

		identity := rfRuntimeWebIdentity(surface, owner)
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}

		name := rfRuntimeWebName(surface, owner)
		pkg := strings.TrimSpace(owner.Package)
		process := strings.TrimSpace(owner.Process)
		item := catalogItem{
			ID:             rfRuntimeWebID(identity),
			Kind:           "integration",
			Name:           name,
			Category:       "Detected Web UI",
			Description:    rfRuntimeWebDescription(pkg, process, surface.Port),
			Source:         "runtime-local",
			State:          "installed_external",
			Installed:      true,
			Enabled:        true,
			ServiceRunning: true,
			Web: &catalogWebMetadata{
				Scheme: surface.Scheme,
				Port:   surface.Port,
				Path:   rfRuntimeWebPath(surface.Path),
				Mode:   "probe-required",
				Embed:  true,
			},
			WebPort:       surface.Port,
			WebPortSource: "runtime-listener",
			Capabilities:  []string{"detect", "open-ui", "runtime-web-discovery"},
			Compatibility: catalogCompatibility{
				Status: "runtime-local",
				Hints:  []string{"Local TCP listener", "HTTP(S) UI detected at runtime"},
			},
			Trust: catalogTrust{
				Status:     "runtime-local",
				ReviewedBy: "routerforge-runtime",
				Note:       "Detected from a local TCP listener and an HTTP(S) UI response; no manifest trust is implied.",
			},
			Actions: catalogActions{
				Reason: "Runtime-discovered Web UI is unmanaged; lifecycle actions are disabled.",
			},
			RegistrySource: "runtime-web-discovery",
			Presentation: map[string]any{
				"runtime_discovered": true,
				"probe_status_code":  surface.StatusCode,
				"probe_content_type": surface.ContentType,
				"probe_title":        surface.Title,
			},
			WebProbeHost: probeHost,
		}

		if pkg != "" {
			item.Detection.Packages = []string{pkg}
			item.PackageInstalled = true
			if version := strings.TrimSpace(installed[pkg]); version != "" {
				item.Version = version
				item.VersionSource = "installed-package"
			}
		}
		if process != "" {
			item.ProcessNames = []string{process}
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Name != items[j].Name {
			return items[i].Name < items[j].Name
		}
		return items[i].ID < items[j].ID
	})
	return items
}

func rfPreferredRuntimeWebOwner(owners []rfWebListenerOwner) rfWebListenerOwner {
	if len(owners) == 0 {
		return rfWebListenerOwner{}
	}
	for _, owner := range owners {
		if strings.TrimSpace(owner.Package) != "" {
			return owner
		}
	}
	for _, owner := range owners {
		if strings.TrimSpace(owner.Process) != "" {
			return owner
		}
	}
	return owners[0]
}

func rfRuntimeWebSelfSurface(surface rfDetectedWebSurface, owner rfWebListenerOwner) bool {
	pkg := strings.ToLower(strings.TrimSpace(owner.Package))
	process := strings.ToLower(strings.TrimSpace(owner.Process))
	executable := strings.ToLower(strings.TrimSpace(owner.Executable))
	if pkg == "routerforge-core" || process == "routerforge" || strings.HasSuffix(executable, "/opt/bin/routerforge") {
		return true
	}
	return pkg == "" && process == "" && strings.Contains(strings.ToLower(strings.TrimSpace(surface.Title)), "routerforge")
}

func rfRuntimeWebMatchesKnown(owner rfWebListenerOwner, port int, packages, processes map[string]struct{}, ports map[int]struct{}) bool {
	pkg := strings.ToLower(strings.TrimSpace(owner.Package))
	if pkg != "" {
		if _, ok := packages[pkg]; ok {
			return true
		}
	}
	process := strings.ToLower(strings.TrimSpace(owner.Process))
	if process != "" {
		if _, ok := processes[process]; ok {
			return true
		}
	}
	if pkg == "" && process == "" {
		_, ok := ports[port]
		return ok
	}
	return false
}

func rfRuntimeWebProbeHost(surface rfDetectedWebSurface) string {
	parsed, err := url.Parse(surface.ProbeURL)
	if err == nil {
		if host := net.ParseIP(strings.Trim(parsed.Hostname(), "[]")); host != nil && !host.IsUnspecified() {
			return host.String()
		}
	}
	address := net.ParseIP(strings.Trim(surface.Address, "[]"))
	if address == nil {
		return ""
	}
	if address.IsUnspecified() {
		if address.To4() != nil {
			return "127.0.0.1"
		}
		return "::1"
	}
	return address.String()
}

func rfRuntimeWebIdentity(surface rfDetectedWebSurface, owner rfWebListenerOwner) string {
	pkg := strings.ToLower(strings.TrimSpace(owner.Package))
	process := strings.ToLower(strings.TrimSpace(owner.Process))
	ownerKey := pkg
	if ownerKey == "" {
		ownerKey = process
	}
	if ownerKey == "" {
		ownerKey = "unknown"
	}
	return strings.Join([]string{ownerKey, strings.ToLower(surface.Scheme), strconv.Itoa(surface.Port)}, "|")
}

func rfRuntimeWebID(identity string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(identity))
	return fmt.Sprintf("runtime-web-%016x", h.Sum64())
}

func rfRuntimeWebName(surface rfDetectedWebSurface, owner rfWebListenerOwner) string {
	if title := strings.TrimSpace(surface.Title); title != "" {
		return title
	}
	if pkg := strings.TrimSpace(owner.Package); pkg != "" {
		return pkg
	}
	if process := strings.TrimSpace(owner.Process); process != "" {
		return process
	}
	return fmt.Sprintf("Web UI :%d", surface.Port)
}

func rfRuntimeWebDescription(pkg, process string, port int) string {
	owner := pkg
	if owner == "" {
		owner = process
	}
	if owner == "" {
		return fmt.Sprintf("Automatically detected local Web UI on TCP port %d.", port)
	}
	return fmt.Sprintf("Automatically detected local Web UI for %s on TCP port %d.", owner, port)
}

func rfRuntimeWebPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\r\n") {
		return "/"
	}
	return path
}

func rfCloneCatalogItems(items []catalogItem) []catalogItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]catalogItem, len(items))
	copy(out, items)
	for i := range out {
		out[i].Capabilities = append([]string(nil), out[i].Capabilities...)
		out[i].Detection.Packages = append([]string(nil), out[i].Detection.Packages...)
		out[i].ProcessNames = append([]string(nil), out[i].ProcessNames...)
		out[i].Compatibility.Hints = append([]string(nil), out[i].Compatibility.Hints...)
		if out[i].Web != nil {
			web := *out[i].Web
			out[i].Web = &web
		}
		if out[i].Presentation != nil {
			presentation := make(map[string]any, len(out[i].Presentation))
			for key, value := range out[i].Presentation {
				presentation[key] = value
			}
			out[i].Presentation = presentation
		}
	}
	return out
}
