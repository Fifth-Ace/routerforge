package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	keeneticRCIBase       = "http://127.0.0.1:79/rci"
	keeneticReadTimeout   = 4 * time.Second
	keeneticOutputMax     = 64 << 10
	keeneticPolicyMax     = 256 << 10
	keeneticInterfaceRoot = "/sys/class/net"
)

type keeneticInterfaceInfo struct {
	Name      string `json:"name"`
	OperState string `json:"oper_state"`
	Up        bool   `json:"up"`
	DefaultV4 bool   `json:"default_v4"`
}

type keeneticIntegrationSnapshot struct {
	Detected          bool                    `json:"detected"`
	ReadOnly          bool                    `json:"read_only"`
	NDMCAvailable     bool                    `json:"ndmc_available"`
	RCIAvailable      bool                    `json:"rci_available"`
	Model             string                  `json:"model,omitempty"`
	Version           string                  `json:"version,omitempty"`
	DefaultV4         string                  `json:"default_ipv4_interface,omitempty"`
	ConfigISP         string                  `json:"config_isp_interface,omitempty"`
	ConfigISPMatches  bool                    `json:"config_isp_matches_default"`
	Interfaces        []keeneticInterfaceInfo `json:"interfaces"`
	EntwareRoot       string                  `json:"entware_root"`
	EntwareMounted    bool                    `json:"entware_mounted"`
	EntwareDevice     string                  `json:"entware_device,omitempty"`
	EntwareFSType     string                  `json:"entware_fs_type,omitempty"`
	EntwareTotalBytes uint64                  `json:"entware_total_bytes,omitempty"`
	EntwareFreeBytes  uint64                  `json:"entware_free_bytes,omitempty"`
	OpkgAvailable     bool                    `json:"opkg_available"`
	PolicyIdentity    string                  `json:"policy_identity,omitempty"`
	PolicyShape       string                  `json:"policy_shape,omitempty"`
	PolicyItems       int                     `json:"policy_items,omitempty"`
	Warnings          []string                `json:"warnings"`
}

func registerKeeneticRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/keenetic", getOnly(handleKeeneticSnapshot))
}

func handleKeeneticSnapshot(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readKeeneticIntegrationSnapshot())
}

func readKeeneticIntegrationSnapshot() keeneticIntegrationSnapshot {
	snapshot := keeneticIntegrationSnapshot{
		ReadOnly:      true,
		EntwareRoot:   "/opt",
		Interfaces:    []keeneticInterfaceInfo{},
		Warnings:      []string{},
		OpkgAvailable: executableFile("/opt/bin/opkg"),
	}

	defaultV4, routeErr := readDefaultIPv4Interface("/proc/net/route")
	if routeErr != nil {
		snapshot.Warnings = append(snapshot.Warnings, "default IPv4 route: "+routeErr.Error())
	}
	snapshot.DefaultV4 = defaultV4
	snapshot.Interfaces = readKeeneticInterfaces(defaultV4)

	if config, _, err := readBoundedFile(configPath, configMaxBytes); err == nil {
		snapshot.ConfigISP = configAssignment(string(config), "ISP_INTERFACE")
	}
	if snapshot.ConfigISP != "" && snapshot.DefaultV4 != "" {
		snapshot.ConfigISPMatches = snapshot.ConfigISP == snapshot.DefaultV4
	}

	if device, fsType, ok := mountForPath("/opt"); ok {
		snapshot.EntwareMounted = true
		snapshot.EntwareDevice = device
		snapshot.EntwareFSType = fsType
	}
	if total, free, err := filesystemSpace("/opt"); err == nil {
		snapshot.EntwareTotalBytes = total
		snapshot.EntwareFreeBytes = free
	}

	if _, err := exec.LookPath("ndmc"); err == nil {
		snapshot.NDMCAvailable = true
		model, version, err := readNDMCVersion()
		if err != nil {
			snapshot.Warnings = append(snapshot.Warnings, "ndmc show version: "+err.Error())
		} else {
			snapshot.Model = model
			snapshot.Version = version
		}
	}

	identity, shape, items, err := readKeeneticPolicyIdentity()
	if err == nil {
		snapshot.RCIAvailable = true
		snapshot.PolicyIdentity = identity
		snapshot.PolicyShape = shape
		snapshot.PolicyItems = items
	} else {
		snapshot.Warnings = append(snapshot.Warnings, "RCI /show/ip/policy: "+err.Error())
	}

	snapshot.Detected = snapshot.NDMCAvailable || snapshot.RCIAvailable
	sort.Slice(snapshot.Interfaces, func(i, j int) bool {
		if snapshot.Interfaces[i].DefaultV4 != snapshot.Interfaces[j].DefaultV4 {
			return snapshot.Interfaces[i].DefaultV4
		}
		return snapshot.Interfaces[i].Name < snapshot.Interfaces[j].Name
	})
	return snapshot
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0111 != 0
}

func readDefaultIPv4Interface(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", errors.New("route table is empty")
	}

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil {
			continue
		}
		if flags&0x1 == 0 {
			continue
		}
		iface := strings.TrimSpace(fields[0])
		if iface != "" && filepath.Base(iface) == iface {
			return iface, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func readKeeneticInterfaces(defaultV4 string) []keeneticInterfaceInfo {
	entries, err := os.ReadDir(keeneticInterfaceRoot)
	if err != nil {
		return []keeneticInterfaceInfo{}
	}
	out := make([]keeneticInterfaceInfo, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		name := entry.Name()
		if name == "" || filepath.Base(name) != name {
			continue
		}
		stateRaw, _ := os.ReadFile(filepath.Join(keeneticInterfaceRoot, name, "operstate"))
		state := strings.TrimSpace(string(stateRaw))
		if state == "" {
			state = "unknown"
		}
		out = append(out, keeneticInterfaceInfo{
			Name:      name,
			OperState: state,
			Up:        state == "up" || state == "unknown",
			DefaultV4: name == defaultV4,
		})
	}
	return out
}

func mountForPath(target string) (string, string, bool) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return "", "", false
	}
	defer file.Close()

	cleanTarget := filepath.Clean(target)
	bestMount := ""
	device := ""
	fsType := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		mountPoint := strings.ReplaceAll(fields[1], `\040`, " ")
		mountPoint = filepath.Clean(mountPoint)
		if cleanTarget != mountPoint && !strings.HasPrefix(cleanTarget+string(os.PathSeparator), mountPoint+string(os.PathSeparator)) {
			continue
		}
		if len(mountPoint) >= len(bestMount) {
			bestMount = mountPoint
			device = fields[0]
			fsType = fields[2]
		}
	}
	return device, fsType, bestMount != ""
}

func filesystemSpace(path string) (uint64, uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	blockSize := uint64(stat.Bsize)
	return stat.Blocks * blockSize, stat.Bavail * blockSize, nil
}

func readNDMCVersion() (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), keeneticReadTimeout)
	defer cancel()
	output, err := safety.RunCommand(ctx, keeneticOutputMax, "ndmc", "-c", "show version")
	if ctx.Err() != nil {
		return "", "", errors.New("timeout")
	}
	if err != nil {
		return "", "", err
	}
	return parseNDMCVersion(string(output))
}

func parseNDMCVersion(text string) (string, string, error) {
	model := ""
	version := ""
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		lower := strings.ToLower(key)
		switch {
		case model == "" && (lower == "model" || lower == "model name" || lower == "device"):
			model = value
		case version == "" && (lower == "release" || lower == "version" || lower == "ndm version"):
			version = value
		}
	}
	if model == "" && version == "" {
		return "", "", errors.New("model/version fields not found")
	}
	return model, version, nil
}

func splitKeyValue(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	for _, sep := range []string{":", "="} {
		if index := strings.Index(line, sep); index > 0 {
			key := strings.TrimSpace(line[:index])
			value := strings.TrimSpace(line[index+1:])
			if key != "" && value != "" {
				return key, value, true
			}
		}
	}
	return "", "", false
}

func readKeeneticPolicyIdentity() (string, string, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), keeneticReadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keeneticRCIBase+"/show/ip/policy", nil)
	if err != nil {
		return "", "", 0, err
	}
	client := &http.Client{
		Timeout: keeneticReadTimeout,
		Transport: &http.Transport{
			Proxy: nil,
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", 0, errors.New(resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, keeneticPolicyMax+1))
	if err != nil {
		return "", "", 0, err
	}
	if len(body) > keeneticPolicyMax {
		return "", "", 0, errors.New("policy response exceeds safety limit")
	}
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return "", "", 0, errors.New("policy response is not JSON")
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", "", 0, err
	}
	sum := sha256.Sum256(canonical)
	shape, items := JSONShape(value)
	return "sha256:" + hex.EncodeToString(sum[:]), shape, items, nil
}

func JSONShape(value any) (string, int) {
	switch typed := value.(type) {
	case []any:
		return "array", len(typed)
	case map[string]any:
		return "object", len(typed)
	default:
		return "scalar", 1
	}
}

func configAssignment(config, key string) string {
	prefix := key + "="
	for _, line := range strings.Split(config, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || !strings.HasPrefix(trimmed, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		value = strings.Trim(value, `"'`)
		return value
	}
	return ""
}
