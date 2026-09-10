package main

import (
	"bufio"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type adminFileVolume struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Mount    string `json:"mount"`
	Device   string `json:"device,omitempty"`
	FSType   string `json:"fs_type,omitempty"`
	ReadOnly bool   `json:"read_only"`
	Kind     string `json:"kind"`
}

var adminFileVolumes = discoverAdminFileVolumes()

func init() {
	seen := make(map[string]bool, len(adminFileAllowedRoots))
	for _, root := range adminFileAllowedRoots {
		seen[filepath.Clean(root)] = true
	}
	for _, volume := range adminFileVolumes {
		root := filepath.Clean(volume.Mount)
		if !seen[root] {
			adminFileAllowedRoots = append(adminFileAllowedRoots, root)
			seen[root] = true
		}
	}
}

func discoverAdminFileVolumes() []adminFileVolume {
	volumes := []adminFileVolume{
		{ID: "entware", Label: "Entware", Mount: "/opt", Kind: "entware"},
		{ID: "temp", Label: "Temporary", Mount: "/tmp", Kind: "temporary"},
		{ID: "system", Label: "System", Mount: "/", Kind: "system", ReadOnly: true},
	}

	file, err := os.Open("/proc/mounts")
	if err != nil {
		return volumes
	}
	defer file.Close()

	known := map[string]bool{"/opt": true, "/tmp": true, "/": true}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		device := unescapeProcMount(fields[0])
		mount := filepath.Clean(unescapeProcMount(fields[1]))
		fsType := fields[2]
		options := fields[3]

		if known[mount] || !adminFileStorageMountCandidate(mount, fsType) {
			continue
		}
		info, err := os.Stat(mount)
		if err != nil || !info.IsDir() {
			continue
		}
		known[mount] = true
		volumes = append(volumes, adminFileVolume{
			ID:       "mount:" + mount,
			Label:    adminFileVolumeLabel(device, mount),
			Mount:    mount,
			Device:   device,
			FSType:   fsType,
			ReadOnly: mountOptionsReadOnly(options),
			Kind:     "storage",
		})
	}

	storage := volumes[3:]
	sort.SliceStable(storage, func(i, j int) bool {
		return storage[i].Mount < storage[j].Mount
	})
	return volumes
}

func unescapeProcMount(value string) string {
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	)
	return replacer.Replace(value)
}

func adminFileStorageMountCandidate(mount, fsType string) bool {
	if mount == "" || !filepath.IsAbs(mount) {
		return false
	}
	switch fsType {
	case "proc", "sysfs", "devtmpfs", "devpts", "tmpfs", "cgroup", "cgroup2",
		"pstore", "debugfs", "tracefs", "securityfs", "configfs", "ramfs":
		return false
	}
	return strings.HasPrefix(mount, "/tmp/mnt/") ||
		strings.HasPrefix(mount, "/mnt/") ||
		strings.HasPrefix(mount, "/media/") ||
		mount == "/storage" ||
		strings.HasPrefix(mount, "/storage/")
}

func adminFileVolumeLabel(device, mount string) string {
	if device != "" && device != "none" {
		name := filepath.Base(device)
		if name != "." && name != "/" && name != "" {
			return name + " · " + mount
		}
	}
	return mount
}

func mountOptionsReadOnly(options string) bool {
	for _, option := range strings.Split(options, ",") {
		if option == "ro" {
			return true
		}
	}
	return false
}

func adminFileRootReadOnly(root string) bool {
	root = filepath.Clean(root)
	if root == "/" {
		return true
	}
	for _, volume := range adminFileVolumes {
		if filepath.Clean(volume.Mount) == root {
			return volume.ReadOnly
		}
	}
	return false
}

func adminFileForbiddenSystemPath(path string) bool {
	path = filepath.Clean(path)
	for _, root := range []string{"/proc", "/sys", "/dev", "/run"} {
		if adminFilePathWithinRoot(root, path) {
			return true
		}
	}
	return false
}

func handleAdminFileVolumes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"volumes": adminFileVolumes,
	})
}
