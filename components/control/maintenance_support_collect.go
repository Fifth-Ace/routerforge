package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	sb "github.com/Fifth-Ace/routerforge/internal/platform/supportbundle"
)

const supportLogLimit = 64 << 10
const supportStateLimit = (2 << 20) + 8192

// Only fixed local paths. Reject symlinks, special files and unbounded input.
// This runs in the isolated worker: even a blocked filesystem cannot occupy
// the main Admin request indefinitely.
func supportRead(path string, limit int, tail bool) ([]byte, error) {
	fd, e := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() {
		return nil, sb.ErrInvalid
	}
	if tail && st.Size() > int64(limit) {
		if _, e = f.Seek(-int64(limit), io.SeekEnd); e != nil {
			return nil, e
		}
	}
	if !tail && st.Size() > int64(limit) {
		return nil, sb.ErrInvalid
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil {
		return nil, e
	}
	if len(raw) > limit {
		if !tail {
			return nil, sb.ErrInvalid
		}
		raw = raw[:limit]
	}
	return raw, nil
}
func supportNumber(s string) *float64 {
	v, e := strconv.ParseFloat(s, 64)
	if e != nil || v < 0 || math.IsInf(v, 0) || math.IsNaN(v) || v > 9007199254740991 {
		return nil
	}
	return &v
}
func supportFirst(path string) *float64 {
	b, e := supportRead(path, 4096, false)
	if e != nil {
		return nil
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return nil
	}
	return supportNumber(f[0])
}

type supportMemory struct {
	Total uint64 `json:"total_bytes"`
	Used  uint64 `json:"used_bytes"`
}

func supportMemoryInfo(raw []byte) (*supportMemory, *supportMemory) {
	v := map[string]uint64{}
	for _, l := range strings.Split(string(raw), "\n") {
		f := strings.Fields(l)
		if len(f) != 3 || f[2] != "kB" {
			continue
		}
		k := strings.TrimSuffix(f[0], ":")
		switch k {
		case "MemTotal", "MemAvailable", "SwapTotal", "SwapFree":
		default:
			continue
		}
		n, e := strconv.ParseUint(f[1], 10, 64)
		if e == nil && n <= 9007199254740991/1024 {
			v[k] = n * 1024
		}
	}
	pair := func(t, a string, zero bool) *supportMemory {
		total, ok := v[t]
		avail, ok2 := v[a]
		if !ok || !ok2 || avail > total || (!zero && total == 0) {
			return nil
		}
		return &supportMemory{total, total - avail}
	}
	return pair("MemTotal", "MemAvailable", false), pair("SwapTotal", "SwapFree", true)
}

type supportStorage struct {
	ID        string `json:"id"`
	Total     uint64 `json:"total_bytes"`
	Available uint64 `json:"available_bytes"`
}

func supportDisk(path, id string) *supportStorage {
	var fs syscall.Statfs_t
	if syscall.Statfs(path, &fs) != nil || fs.Bsize <= 0 || uint64(fs.Blocks) > 9007199254740991/uint64(fs.Bsize) || uint64(fs.Bavail) > uint64(fs.Blocks) {
		return nil
	}
	return &supportStorage{id, uint64(fs.Blocks) * uint64(fs.Bsize), uint64(fs.Bavail) * uint64(fs.Bsize)}
}
func supportSystem() sb.Entry {
	kernel, _ := supportRead("/proc/sys/kernel/osrelease", 4096, false)
	model, _ := supportRead("/sys/firmware/devicetree/base/model", 4096, false)
	memRaw, _ := supportRead("/proc/meminfo", 64<<10, false)
	memory, swap := supportMemoryInfo(memRaw)
	uptime := supportFirst("/proc/uptime")
	load := supportFirst("/proc/loadavg")
	disks := []supportStorage{}
	for _, p := range []struct{ path, id string }{{"/opt", "entware"}, {"/tmp", "temporary"}} {
		if d := supportDisk(p.path, p.id); d != nil {
			disks = append(disks, *d)
		}
	}
	// The model is reduced to a KN-xxxx code, kernel to numeric version.
	raw := sb.JSON(map[string]any{"architecture": sb.Architecture(runtime.GOARCH), "model": sb.Model(string(model)), "kernel": sb.Kernel(string(kernel)), "admin_version": sb.Version(version), "cpu_count": runtime.NumCPU(), "uptime_sec": uptime, "load_1": load, "memory": memory, "swap": swap, "storage": disks})
	state, reason := "ok", ""
	if memory == nil || uptime == nil || load == nil || len(disks) != 2 {
		state, reason = "partial", "PARTIAL"
	}
	return sb.Entry{ID: "system", State: state, Reason: reason, Data: raw}
}
func supportPackages() sb.Entry {
	for _, p := range []string{"/opt/lib/opkg/status", "/opt/var/opkg/status"} {
		if b, e := supportRead(p, 2<<20, false); e == nil {
			rows := sb.Packages(b)
			if len(rows) > 0 {
				return sb.Entry{ID: "packages", State: "ok", Data: sb.JSON(rows)}
			}
		}
	}
	return sb.Entry{ID: "packages", State: "skipped", Reason: "NOT_AVAILABLE"}
}

func supportModuleHealth(ctx context.Context, id string) sb.Health {
	// No arbitrary URL, no network sockets and no mutating methods. Security
	// modules are intentionally not queried by this new integration.
	known := false
	for _, v := range sb.ProbeIDs() {
		if id == v {
			known = true
		}
	}
	if !known {
		return sb.ProjectHealth(id, nil)
	}
	socket := "/opt/var/run/routerforge-" + id + ".sock"
	path := "/v1/health"
	if id == "dns-router" {
		path = "/v1/status"
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	client := http.Client{Transport: tr, Timeout: 1500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix"+path, nil)
	if e != nil {
		return sb.ProjectHealth(id, nil)
	}
	resp, e := client.Do(req)
	if e != nil {
		return sb.ProjectHealth(id, nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return sb.ProjectHealth(id, nil)
	}
	raw, e := io.ReadAll(io.LimitReader(resp.Body, sb.MaxEntry+1))
	if e != nil {
		return sb.ProjectHealth(id, nil)
	}
	return sb.ProjectHealth(id, raw)
}
func supportModules(ctx context.Context, modules []string) sb.Entry {
	rows := []sb.Health{}
	state, reason := "ok", ""
	for _, id := range modules {
		h := supportModuleHealth(ctx, id)
		if h.State == "unavailable" {
			state, reason = "partial", "PARTIAL"
		}
		rows = append(rows, h)
	}
	return sb.Entry{ID: "modules", State: state, Reason: reason, Data: sb.JSON(rows)}
}

type supportCatalogueCounts struct {
	Generation  int64 `json:"generation"`
	Devices     int   `json:"devices"`
	Connections int   `json:"connections"`
	Policies    int   `json:"check_policies"`
	Objects     int   `json:"policy_objects"`
	History     int   `json:"history"`
}
type supportBindingCounts struct {
	Generation int64 `json:"generation"`
	Count      int   `json:"count"`
	Pending    int   `json:"pending"`
}

func supportCatalogue(raw []byte) *supportCatalogueCounts {
	var s struct {
		Current struct {
			Generation *int64 `json:"generation"`
			Catalog    struct {
				SchemaVersion                                      int `json:"schemaVersion"`
				Devices, Connections, CheckPolicies, PolicyObjects []json.RawMessage
			} `json:"catalog"`
		} `json:"current"`
		History []json.RawMessage `json:"history"`
	}
	if json.Unmarshal(raw, &s) != nil || s.Current.Generation == nil || *s.Current.Generation < 0 || *s.Current.Generation > 9007199254740991 || s.Current.Catalog.SchemaVersion != 1 || len(s.History) > 4 {
		return nil
	}
	c := s.Current.Catalog
	if c.Devices == nil || c.Connections == nil || c.CheckPolicies == nil || c.PolicyObjects == nil {
		return nil
	}
	return &supportCatalogueCounts{*s.Current.Generation, len(c.Devices), len(c.Connections), len(c.CheckPolicies), len(c.PolicyObjects), len(s.History)}
}
func supportBindings(raw []byte) *supportBindingCounts {
	var s struct {
		SchemaVersion int    `json:"schemaVersion"`
		Generation    *int64 `json:"generation"`
		Bindings      []struct {
			Pending json.RawMessage `json:"pending"`
		} `json:"bindings"`
	}
	if json.Unmarshal(raw, &s) != nil || s.SchemaVersion != 1 || s.Generation == nil || *s.Generation < 0 || *s.Generation > 9007199254740991 || s.Bindings == nil || len(s.Bindings) > 16 {
		return nil
	}
	p := 0
	for _, b := range s.Bindings {
		if len(b.Pending) > 0 && string(b.Pending) != "null" {
			p++
		}
	}
	return &supportBindingCounts{*s.Generation, len(s.Bindings), p}
}
func supportFoundation(obs sb.Observation) sb.Entry {
	var c *supportCatalogueCounts
	var b *supportBindingCounts
	cs, bs := "unavailable", "unavailable"
	raw, e := supportRead("/opt/etc/routerforge/policy-catalog-v1.json", supportStateLimit, false)
	if errors.Is(e, os.ErrNotExist) {
		c = &supportCatalogueCounts{}
		cs = "not_created"
	} else if e == nil {
		c = supportCatalogue(raw)
		if c != nil {
			cs = "saved_metadata"
		}
	}
	raw, e = supportRead("/opt/etc/routerforge/policy-bindings-v1.json", supportStateLimit, false)
	if errors.Is(e, os.ErrNotExist) {
		b = &supportBindingCounts{}
		bs = "not_created"
	} else if e == nil {
		b = supportBindings(raw)
		if b != nil {
			bs = "saved_metadata"
		}
	}
	state, reason := "ok", ""
	if c == nil || b == nil || !obs.Available {
		state, reason = "partial", "PARTIAL"
	}
	return sb.Entry{ID: "foundation", State: state, Reason: reason, Data: sb.JSON(map[string]any{"catalogue_state": cs, "catalogue": c, "bindings_state": bs, "bindings": b, "core_observation": obs, "independent_samples": true, "metadata_only": true})}
}
func supportLogs() sb.Entry {
	for _, p := range []string{"/opt/var/log/routerforge.log", "/tmp/routerforge/logs/routerforge.log"} {
		if raw, e := supportRead(p, supportLogLimit, true); e == nil {
			return sb.Entry{ID: "logs", State: "ok", Data: sb.JSON(sb.SummarizeLogs(raw))}
		}
	}
	return sb.Entry{ID: "logs", State: "skipped", Reason: "NO_LOG"}
}

type supportWorkerRequest struct {
	Sections    []string       `json:"sections"`
	Modules     []string       `json:"modules"`
	Observation sb.Observation `json:"observation"`
}

func supportCollectWorker(in io.Reader, out io.Writer) error {
	raw, e := io.ReadAll(io.LimitReader(in, 8193))
	if e != nil || len(raw) > 8192 {
		return sb.ErrInvalid
	}
	var q supportWorkerRequest
	if json.Unmarshal(raw, &q) != nil {
		return sb.ErrInvalid
	}
	sel, e := sb.Selection(q.Sections)
	if e != nil {
		return e
	}
	mods, e := sb.SelectedModules(sel, q.Modules)
	if e != nil {
		return e
	}
	obsRaw, _ := json.Marshal(q.Observation)
	obs := sb.ParseObservation(string(obsRaw))
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	entries := []sb.Entry{}
	for _, id := range sel {
		switch id {
		case "system":
			entries = append(entries, supportSystem())
		case "packages":
			entries = append(entries, supportPackages())
		case "modules":
			entries = append(entries, supportModules(ctx, mods))
		case "foundation":
			entries = append(entries, supportFoundation(obs))
		case "logs":
			entries = append(entries, supportLogs())
		}
	}
	return json.NewEncoder(out).Encode(entries)
}
