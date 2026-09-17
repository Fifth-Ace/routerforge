package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var opkgInfoDirs = []string{
	"/opt/lib/opkg/info",
	"/opt/var/opkg/info",
}

type serviceInspectorPackage struct {
	Name         string `json:"name"`
	Version      string `json:"version,omitempty"`
	Architecture string `json:"architecture,omitempty"`
	Status       string `json:"status,omitempty"`
	Source       string `json:"source"`
}

type serviceInspectorProcess struct {
	PID      int    `json:"pid"`
	Name     string `json:"name"`
	Command  string `json:"command"`
	State    string `json:"state"`
	User     string `json:"user"`
	UID      int    `json:"uid"`
	RSSKB    int64  `json:"rss_kb"`
	VmSizeKB int64  `json:"vmsize_kb"`
	Threads  int    `json:"threads"`
	Match    string `json:"match"`
}

type serviceInspectorEvidence struct {
	PackageSource string   `json:"package_source,omitempty"`
	ProcessMatch  []string `json:"process_match,omitempty"`
	PortSource    string   `json:"port_source,omitempty"`
	ConfigSource  string   `json:"config_source,omitempty"`
	LogSource     string   `json:"log_source,omitempty"`
}

type serviceInspectorResource struct {
	ProcessCount int   `json:"process_count"`
	RSSKB        int64 `json:"rss_kb"`
	VmSizeKB     int64 `json:"vmsize_kb"`
	Threads      int   `json:"threads"`
	Listeners    int   `json:"listeners"`
}

type serviceInspectorItem struct {
	ID             string                    `json:"id"`
	Name           string                    `json:"name"`
	InitScript     string                    `json:"init_script"`
	Executable     bool                      `json:"executable"`
	Running        bool                      `json:"running"`
	RunningSource  string                    `json:"running_source,omitempty"`
	Package        *serviceInspectorPackage  `json:"package,omitempty"`
	Processes      []serviceInspectorProcess `json:"processes"`
	ListeningPorts []portInfo                `json:"listening_ports"`
	ConfigPaths    []string                  `json:"config_paths"`
	LogPaths       []string                  `json:"log_paths"`
	Resource       serviceInspectorResource  `json:"resource"`
	Evidence       serviceInspectorEvidence  `json:"evidence"`
	ModifiedAt     time.Time                 `json:"modified_at"`
}

type serviceInspectorResponse struct {
	GeneratedAt time.Time              `json:"generated_at"`
	ReadOnly    bool                   `json:"read_only"`
	Services    []serviceInspectorItem `json:"services"`
}

type packageOwnership struct {
	Package packageInfo
	List    string
	Files   []string
}

func readServiceInspector() serviceInspectorResponse {
	services := readServices()
	packages := readPackages()
	owners := readPackageOwnership(packages)
	processes := readInspectorProcesses()
	ports := readPorts()

	items := make([]serviceInspectorItem, 0, len(services))
	for _, service := range services {
		item := inspectService(service, owners, processes, ports)
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return serviceInspectorResponse{
		GeneratedAt: time.Now(),
		ReadOnly:    true,
		Services:    items,
	}
}

func readPackageOwnership(packages []packageInfo) map[string]packageOwnership {
	byName := make(map[string]packageInfo, len(packages))
	for _, pkg := range packages {
		byName[pkg.Name] = pkg
	}

	out := map[string]packageOwnership{}
	for _, dir := range opkgInfoDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".list") {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), ".list")
			if _, exists := out[name]; exists {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			files := readPackageList(path)
			if len(files) == 0 {
				continue
			}
			out[name] = packageOwnership{
				Package: byName[name],
				List:    path,
				Files:   files,
			}
		}
	}
	return out
}

func readPackageList(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		value := strings.TrimSpace(sc.Text())
		if value == "" || !filepath.IsAbs(value) {
			continue
		}
		out = append(out, filepath.Clean(value))
	}
	sort.Strings(out)
	return uniqueStrings(out)
}

func inspectService(
	service serviceInfo,
	owners map[string]packageOwnership,
	processes []processInfo,
	ports []portInfo,
) serviceInspectorItem {
	item := serviceInspectorItem{
		ID:             service.ID,
		Name:           service.Name,
		InitScript:     service.Path,
		Executable:     service.Executable,
		Running:        service.Running,
		RunningSource:  service.RunningSource,
		Processes:      []serviceInspectorProcess{},
		ListeningPorts: []portInfo{},
		ConfigPaths:    []string{},
		LogPaths:       []string{},
		ModifiedAt:     service.ModifiedAt,
	}

	owner, ownerOK := packageForService(service.Path, owners)
	var packageFiles []string
	if ownerOK {
		pkg := owner.Package
		if pkg.Name == "" {
			pkg.Name = packageNameFromList(owner.List)
		}
		item.Package = &serviceInspectorPackage{
			Name:         pkg.Name,
			Version:      pkg.Version,
			Architecture: pkg.Architecture,
			Status:       pkg.Status,
			Source:       owner.List,
		}
		item.Evidence.PackageSource = "opkg-info-list"
		packageFiles = owner.Files
		item.ConfigPaths, item.LogPaths = classifyPackageEvidence(owner.Files, service.Path)
		if len(item.ConfigPaths) > 0 {
			item.Evidence.ConfigSource = "package-file-list"
		}
		if len(item.LogPaths) > 0 {
			item.Evidence.LogSource = "package-file-list"
		}
	}

	candidates := serviceMatchCandidates(service, packageFiles)
	matched, evidence := matchServiceProcesses(processes, candidates)
	item.Evidence.ProcessMatch = evidence

	pids := make(map[int]bool, len(matched))
	for _, process := range matched {
		match := processMatchReason(process, candidates)
		item.Processes = append(item.Processes, serviceInspectorProcess{
			PID:      process.PID,
			Name:     process.Name,
			Command:  process.Command,
			State:    process.State,
			User:     process.User,
			UID:      process.UID,
			RSSKB:    process.RSSKB,
			VmSizeKB: process.VmSizeKB,
			Threads:  process.Threads,
			Match:    match,
		})
		pids[process.PID] = true
		item.Resource.RSSKB += process.RSSKB
		item.Resource.VmSizeKB += process.VmSizeKB
		item.Resource.Threads += process.Threads
	}
	item.Resource.ProcessCount = len(item.Processes)

	for _, port := range ports {
		if port.PID <= 0 || !pids[port.PID] {
			continue
		}
		item.ListeningPorts = append(item.ListeningPorts, port)
	}
	if len(item.ListeningPorts) > 0 {
		item.Evidence.PortSource = "proc-net+fd-inode"
	}
	item.Resource.Listeners = len(item.ListeningPorts)

	// Inspector evidence is more specific than the legacy process-name boolean.
	// Keep legacy Running as a compatibility fallback, but promote exact evidence
	// when processes are correlated.
	if len(item.Processes) > 0 {
		item.Running = true
		item.RunningSource = "service-inspector-process-evidence"
	}

	return item
}

func packageForService(initScript string, owners map[string]packageOwnership) (packageOwnership, bool) {
	target := filepath.Clean(initScript)
	names := make([]string, 0, len(owners))
	for name := range owners {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		owner := owners[name]
		for _, path := range owner.Files {
			if filepath.Clean(path) == target {
				return owner, true
			}
		}
	}
	return packageOwnership{}, false
}

func packageNameFromList(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".list")
}

func classifyPackageEvidence(files []string, initScript string) ([]string, []string) {
	var configs []string
	var logs []string
	initScript = filepath.Clean(initScript)

	for _, raw := range files {
		path := filepath.Clean(raw)
		if path == initScript {
			continue
		}
		lower := strings.ToLower(path)
		switch {
		case strings.HasPrefix(lower, "/opt/etc/"),
			strings.HasPrefix(lower, "/etc/"):
			if !strings.Contains(lower, "/init.d/") {
				configs = append(configs, path)
			}
		case strings.HasPrefix(lower, "/opt/var/log/"),
			strings.HasPrefix(lower, "/var/log/"):
			logs = append(logs, path)
		}
	}

	sort.Strings(configs)
	sort.Strings(logs)
	return uniqueStrings(configs), uniqueStrings(logs)
}

func readInspectorProcesses() []processInfo {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var out []processInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if process, ok := readProcessByPID(pid); ok {
			out = append(out, process)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

func serviceMatchCandidates(service serviceInfo, packageFiles []string) []string {
	values := []string{
		service.Name,
		trimServicePrefix(service.ID),
	}

	for _, path := range packageFiles {
		base := filepath.Base(path)
		lower := strings.ToLower(path)
		if base == "" || base == "." || base == "/" {
			continue
		}
		if strings.HasPrefix(lower, "/opt/bin/") ||
			strings.HasPrefix(lower, "/opt/sbin/") ||
			strings.HasPrefix(lower, "/usr/bin/") ||
			strings.HasPrefix(lower, "/usr/sbin/") {
			values = append(values, base)
		}
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeServiceToken(value)
		if len(value) < 3 {
			continue
		}
		out = append(out, value)
	}
	sort.Strings(out)
	return uniqueStrings(out)
}

func matchServiceProcesses(processes []processInfo, candidates []string) ([]processInfo, []string) {
	var matched []processInfo
	var evidence []string

	for _, process := range processes {
		reason := processMatchReason(process, candidates)
		if reason == "" {
			continue
		}
		matched = append(matched, process)
		evidence = append(evidence, "pid="+strconv.Itoa(process.PID)+":"+reason)
	}

	sort.Slice(matched, func(i, j int) bool { return matched[i].PID < matched[j].PID })
	sort.Strings(evidence)
	return matched, uniqueStrings(evidence)
}

func processMatchReason(process processInfo, candidates []string) string {
	name := normalizeServiceToken(process.Name)
	commandBase := normalizeServiceToken(filepath.Base(strings.TrimSpace(process.Command)))

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if name == candidate {
			return "name=" + candidate
		}
		if commandBase == candidate {
			return "command=" + candidate
		}
	}
	return ""
}

func normalizeServiceToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, ".sh")
	value = strings.TrimSuffix(value, ".bin")
	value = strings.ReplaceAll(value, "-", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := values[:0]
	var previous string
	for i, value := range values {
		if i == 0 || value != previous {
			out = append(out, value)
			previous = value
		}
	}
	return out
}
