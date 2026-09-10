package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var moduleSockets = map[string][]string{
	"dns":        {"/opt/var/run/routerforge-dns.sock"},
	"admin":      {"/opt/var/run/routerforge-admin.sock", "/opt/var/run/dns-monitor-admin.sock"},
	"monitoring": {"/opt/var/run/routerforge-monitoring.sock"},
	"system":     {"/opt/var/run/routerforge-system.sock", "/opt/var/run/dns-monitor-system.sock"},
	"thermal":    {"/opt/var/run/routerforge-thermal.sock", "/opt/var/run/dns-monitor-thermal.sock"},
	"storage":    {"/opt/var/run/routerforge-storage.sock", "/opt/var/run/dns-monitor-storage.sock"},
	"network":    {"/opt/var/run/routerforge-network.sock", "/opt/var/run/dns-monitor-network.sock"},
}

var modulePackageNames = map[string]string{
	"dns":        "routerforge-dns",
	"admin":      "routerforge-admin",
	"monitoring": "routerforge-monitoring",
	"system":     "routerforge-monitoring",
	"thermal":    "routerforge-monitoring",
	"storage":    "routerforge-monitoring",
	"network":    "routerforge-monitoring",
}

var moduleInstalledPackages = readInstalledPackages

type moduleProxyContextKey string

const (
	adminMutationAuthorizedKey       moduleProxyContextKey = "admin-mutation-authorized"
	adminMutationAuthorizationHeader                       = "X-RouterForge-Admin-Authorized"
	adminMutationAuthorizationValue                        = "session-root-v1"
)

func moduleMutationAPI(moduleID string) bool {
	return moduleID == "dns" || moduleID == "admin"
}

const (
	dnsModuleMutationBodyLimit     int64 = 64 << 10
	adminModuleMutationBodyLimit   int64 = 8 << 10
	adminFileWriteRequestBodyLimit int64 = 272 << 10
)

func moduleMutationBodyLimit(moduleID string) int64 {
	switch moduleID {
	case "dns":
		return dnsModuleMutationBodyLimit
	case "admin":
		return adminModuleMutationBodyLimit
	default:
		return 0
	}
}

func moduleMutationBodyLimitForRequest(r *http.Request, moduleID string) int64 {
	if moduleID == "admin" && r.Method == http.MethodPost && r.URL.Path == "/api/modules/admin/files/write" {
		return adminFileWriteRequestBodyLimit
	}
	return moduleMutationBodyLimit(moduleID)
}

func boundedModuleMutationRequest(w http.ResponseWriter, r *http.Request, moduleID string) (*http.Request, bool) {
	limit := moduleMutationBodyLimitForRequest(r, moduleID)
	if limit <= 0 || r.Method == http.MethodGet || r.Method == http.MethodHead {
		return r, true
	}

	if r.ContentLength > limit {
		writeModuleMutationBodyTooLarge(w, moduleID, limit)
		return nil, false
	}
	if r.Body == nil || r.Body == http.NoBody {
		return r, true
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if err != nil {
		w.Header().Set("Connection", "close")
		writeModuleJSON(w, http.StatusBadRequest, map[string]any{
			"error":        "invalid RouterForge module mutation request body",
			"module":       moduleID,
			"mutation_api": true,
		})
		return nil, false
	}
	if int64(len(body)) > limit {
		writeModuleMutationBodyTooLarge(w, moduleID, limit)
		return nil, false
	}

	clone := r.Clone(r.Context())
	clone.ContentLength = int64(len(body))
	clone.TransferEncoding = nil
	clone.Trailer = nil
	if len(body) == 0 {
		clone.Body = http.NoBody
		clone.GetBody = func() (io.ReadCloser, error) { return http.NoBody, nil }
	} else {
		clone.Body = io.NopCloser(bytes.NewReader(body))
		clone.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(body)), nil
		}
	}
	return clone, true
}

func writeModuleMutationBodyTooLarge(w http.ResponseWriter, moduleID string, limit int64) {
	w.Header().Set("Connection", "close")
	writeModuleJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
		"error":          "RouterForge module mutation request body exceeds limit",
		"module":         moduleID,
		"mutation_api":   true,
		"max_body_bytes": limit,
	})
}

func adminModuleMutationRequest(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		return false
	}
	const prefix = "/api/modules/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	return len(parts) > 0 && strings.EqualFold(strings.TrimSpace(parts[0]), "admin")
}

func adminModuleFileRequest(r *http.Request) bool {
	const prefix = "/api/modules/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "admin") {
		return false
	}
	return parts[1] == "files" || strings.HasPrefix(parts[1], "files/")
}

func adminModuleTerminalRequest(r *http.Request) bool {
	const prefix = "/api/modules/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "admin") {
		return false
	}
	return parts[1] == "terminal" || strings.HasPrefix(parts[1], "terminal/")
}

func markAdminMutationAuthorized(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), adminMutationAuthorizedKey, true)
	return r.WithContext(ctx)
}

func adminMutationAuthorized(r *http.Request) bool {
	authorized, _ := r.Context().Value(adminMutationAuthorizedKey).(bool)
	return authorized
}

func securedModuleProxy(auth *authManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminMutation := adminModuleMutationRequest(r)
		adminFiles := adminModuleFileRequest(r)
		adminTerminal := adminModuleTerminalRequest(r)
		guarded := adminMutation || adminFiles || adminTerminal
		if guarded {
			if !sameOriginRequest(r) {
				message := "cross-origin Admin mutation rejected"
				if adminFiles && !adminMutation {
					message = "cross-origin Admin file access rejected"
				}
				if adminTerminal && !adminMutation {
					message = "cross-origin Admin terminal access rejected"
				}
				writeModuleJSON(w, http.StatusForbidden, map[string]any{
					"error":        message,
					"mutation_api": adminMutation,
				})
				return
			}
			requireRootSession := (adminMutation && !adminFiles && !adminTerminal) ||
				((adminFiles || adminTerminal) && auth.authRequired())
			if requireRootSession {
				user, authenticated := auth.sessionUser(r)
				if !authenticated || user != "root" {
					message := "authenticated Entware root session required for Admin mutation"
					if adminFiles && !adminMutation {
						message = "authenticated Entware root session required for Admin file access"
					}
					if adminTerminal && !adminMutation {
						message = "authenticated Entware root session required for Admin terminal access"
					}
					writeModuleJSON(w, http.StatusUnauthorized, map[string]any{
						"error":         message,
						"auth_required": true,
						"mutation_api":  adminMutation,
					})
					return
				}
			}
			r = markAdminMutationAuthorized(r)
		}
		proxyModuleAPI(w, r)
	}
}

const moduleReconnectHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>RouterForge module reconnect</title>
  <style>
    html,body{min-height:100%;margin:0;background:transparent;color:inherit;font:inherit}
    body{display:grid;place-items:center;min-height:55vh;padding:2rem;box-sizing:border-box}
    main{text-align:center;max-width:36rem}
    .spinner{width:1.6rem;height:1.6rem;margin:0 auto 1rem;border:.18rem solid currentColor;border-right-color:transparent;border-radius:50%;animation:spin .8s linear infinite;opacity:.75}
    p{margin:.35rem 0;line-height:1.45}
    .muted{opacity:.65}
    @keyframes spin{to{transform:rotate(360deg)}}
    @media (prefers-reduced-motion:reduce){.spinner{animation:none}}
  </style>
</head>
<body>
  <main role="status" aria-live="polite">
    <div class="spinner" aria-hidden="true"></div>
    <p id="state">Модуль RouterForge перезапускается…</p>
    <p class="muted">Переподключение к модулю… / Reconnecting…</p>
  </main>
  <script>
    (function () {
      var state = document.getElementById('state');
      try {
        var hostStyle = window.parent.getComputedStyle(window.parent.document.body);
        document.body.style.color = hostStyle.color;
        document.body.style.fontFamily = hostStyle.fontFamily;
      } catch (_) {}
      var marker = '/ui/';
      var pos = window.location.pathname.indexOf(marker);
      var healthURL = (pos >= 0 ? window.location.pathname.slice(0, pos) : '/api/modules') + '/health';

      function retry() {
        window.setTimeout(probe, 750);
      }

      function probe() {
        fetch(healthURL, {
          cache: 'no-store',
          credentials: 'same-origin',
          headers: { 'Accept': 'application/json' }
        }).then(function (response) {
          if (response.ok) {
            window.location.reload();
            return null;
          }
          if (response.status === 503) {
            return response.json().catch(function () { return null; });
          }
          return null;
        }).then(function (payload) {
          if (payload && payload.installed === false) {
            state.textContent = 'Модуль не установлен / Module is not installed';
            return;
          }
          retry();
        }).catch(retry);
      }

      probe();
    }());
  </script>
</body>
</html>
`

func activeModuleSocket(moduleID string) (string, bool) {
	candidates, ok := moduleSockets[moduleID]
	if !ok || len(candidates) == 0 {
		return "", false
	}
	for _, socket := range candidates {
		if _, err := os.Stat(socket); err == nil {
			return socket, true
		}
	}
	return candidates[0], true
}

func moduleInstalled(moduleID string) bool {
	pkg := strings.TrimSpace(modulePackageNames[moduleID])
	if pkg == "" {
		return false
	}
	_, ok := moduleInstalledPackages()[pkg]
	return ok
}

func moduleMethodAllowed(moduleID, method string) bool {
	if method == http.MethodHead {
		return true
	}
	if moduleID == "dns" {
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete:
			return true
		default:
			return false
		}
	}
	if moduleID == "admin" {
		switch method {
		case http.MethodGet, http.MethodPost:
			return true
		default:
			return false
		}
	}
	return method == http.MethodGet
}

func moduleTargetPath(rest string) (string, bool) {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "/v1/health", true
	}
	if strings.Contains(rest, "..") {
		return "", false
	}
	trailingSlash := strings.HasSuffix(rest, "/")
	clean := path.Clean("/" + rest)
	if clean == "/." || !strings.HasPrefix(clean, "/") {
		return "", false
	}
	// path.Clean intentionally strips the trailing slash. For module UI
	// directories that changes /v1/ui/ into /v1/ui and triggers the module's
	// canonical redirect, leaking the internal /v1 path back to the browser.
	if trailingSlash && clean != "/" {
		clean += "/"
	}
	return "/v1" + clean, true
}

func moduleUIPath(targetPath string) bool {
	return targetPath == "/v1/ui" || strings.HasPrefix(targetPath, "/v1/ui/")
}

func moduleTransport(socket string) *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socket)
		},
	}
}

func proxyModuleAPI(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/modules/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		http.NotFound(w, r)
		return
	}

	moduleID := strings.ToLower(strings.TrimSpace(parts[0]))
	if moduleID == "profiling" {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeModuleJSON(w, http.StatusMethodNotAllowed, map[string]any{
				"error":        "profiling status is read-only",
				"mutation_api": false,
			})
			return
		}
		if len(parts) == 1 || parts[1] == "" || parts[1] == "status" {
			writeModuleJSON(w, http.StatusOK, profilingStatusSnapshot())
			return
		}
		http.NotFound(w, r)
		return
	}

	if !moduleMethodAllowed(moduleID, r.Method) {
		allow := "GET, HEAD"
		if moduleID == "dns" {
			allow = "GET, HEAD, POST, PATCH, DELETE"
		} else if moduleID == "admin" {
			allow = "GET, HEAD, POST"
		}
		w.Header().Set("Allow", allow)
		writeModuleJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"error":        "method is not allowed by this RouterForge module",
			"mutation_api": moduleMutationAPI(moduleID),
		})
		return
	}

	boundedRequest, ok := boundedModuleMutationRequest(w, r, moduleID)
	if !ok {
		return
	}
	r = boundedRequest

	socket, ok := activeModuleSocket(moduleID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	suffix := ""
	if len(parts) == 2 {
		suffix = parts[1]
	}
	targetPath, ok := moduleTargetPath(suffix)
	if !ok {
		http.NotFound(w, r)
		return
	}

	transport := moduleTransport(socket)
	defer transport.CloseIdleConnections()

	upstream := &url.URL{Scheme: "http", Host: "unix"}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Transport = transport
	proxy.FlushInterval = -1
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.URL.Path = targetPath
		req.URL.RawPath = ""
		req.Host = "unix"
		if moduleID == "admin" {
			req.Header.Del(adminMutationAuthorizationHeader)
			if adminMutationAuthorized(r) {
				req.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
			}
		}
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if moduleUIPath(targetPath) {
			resp.Header.Set("Cache-Control", "no-cache")
		} else {
			resp.Header.Set("Cache-Control", "no-store")
		}
		return nil
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		if moduleUIPath(targetPath) {
			writeModuleReconnectHTML(rw)
			return
		}
		moduleUnavailable(rw, moduleID, fmt.Sprintf("%s: %v", socket, err))
	}
	proxy.ServeHTTP(w, r)
}

func readModuleRaw(ctx context.Context, moduleID, targetPath string, timeout time.Duration) ([]byte, error) {
	socket, ok := activeModuleSocket(moduleID)
	if !ok {
		return nil, fmt.Errorf("unknown module %q", moduleID)
	}
	if strings.Contains(targetPath, "..") || !strings.HasPrefix(targetPath, "/v1/") {
		return nil, fmt.Errorf("invalid module target path")
	}
	transport := moduleTransport(socket)
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix"+targetPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, fmt.Errorf("module %s returned http %d", moduleID, resp.StatusCode)
	}
	return body, nil
}

func moduleUnavailable(w http.ResponseWriter, moduleID, detail string) {
	writeModuleJSON(w, http.StatusServiceUnavailable, map[string]any{
		"module":       moduleID,
		"installed":    moduleInstalled(moduleID),
		"running":      false,
		"mutation_api": moduleMutationAPI(moduleID),
		"error":        "RouterForge module is not available",
		"detail":       detail,
	})
}

func writeModuleReconnectHTML(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Retry-After", "1")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = io.WriteString(w, moduleReconnectHTML)
}

func writeModuleJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
