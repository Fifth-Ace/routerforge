package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// Storage Doctor extends the existing read-only Monitoring storage service.
// No disk write, fsck, mount, SMART self-test or daemon is started here.
// SMART I/O is explicit only: the passive doctor endpoint never invokes smartctl.
const doctorMaxIncidents = 64

var doctorDeviceName = regexp.MustCompile(`^(sd[a-z]+|hd[a-z]+|vd[a-z]+|nvme[0-9]+n[0-9]+|mmcblk[0-9]+)$`)

// This ring is intentionally bounded and volatile. Without a persistent event
// source, it must never be presented as the full disconnect history.
var doctorTracker = struct {
	sync.Mutex
	initialized bool
	mounts      map[string]bool
	events      []doctorIncident
}{mounts: map[string]bool{}}

var doctorSmartState = struct {
	sync.Mutex
	active map[string]bool
	cache  map[string]doctorSmartCache
}{active: map[string]bool{}, cache: map[string]doctorSmartCache{}}

type doctorMount struct {
	Device         string  `json:"device"`
	Mount          string  `json:"mount"`
	FSType         string  `json:"fs_type"`
	ReadOnly       bool    `json:"read_only"`
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPct        float64 `json:"used_pct"`
	InodesFree     *uint64 `json:"inodes_free,omitempty"`
	InodesTotal    *uint64 `json:"inodes_total,omitempty"`
}
type doctorDisk struct {
	ID         string        `json:"id"`
	Device     string        `json:"device"`
	Model      string        `json:"model,omitempty"`
	Vendor     string        `json:"vendor,omitempty"`
	Transport  string        `json:"transport,omitempty"`
	SizeBytes  uint64        `json:"size_bytes"`
	Removable  bool          `json:"removable"`
	ReadOnly   bool          `json:"read_only"`
	Entware    bool          `json:"entware"`
	Mounts     []doctorMount `json:"mounts"`
	Partitions []string      `json:"partitions"`
}
type doctorEntware struct {
	Mounted          bool   `json:"mounted"`
	Writable         bool   `json:"writable"`
	Device           string `json:"device,omitempty"`
	FSType           string `json:"fs_type,omitempty"`
	InitPresent      bool   `json:"init_present"`
	PackageDBPresent bool   `json:"package_db_present"`
	Warning          string `json:"warning,omitempty"`
}
type doctorIncident struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Mount  string    `json:"mount"`
	Detail string    `json:"detail"`
}
type doctorView struct {
	Disks                  []doctorDisk                        `json:"disks"`
	Entware                doctorEntware                       `json:"entware"`
	Incidents              []doctorIncident                    `json:"incidents"`
	HistoryScope           string                              `json:"history_scope"`
	KernelHistoryAvailable bool                                `json:"kernel_history_available"`
	History                []doctorHealthEvent                 `json:"history"`
	Temperatures           map[string]doctorTemperatureReading `json:"temperatures"`
	TemperatureHistory     map[string][]doctorTemperaturePoint `json:"temperature_history"`
	SmartctlInstalled      bool                                `json:"smartctl_installed"`
	CollectedAt            time.Time                           `json:"collected_at"`
}

func doctorParent(disk, device string) bool {
	if !strings.HasPrefix(device, "/dev/") {
		return false
	}
	name := filepath.Base(device)
	if name == disk {
		return true
	}
	prefix := disk
	if len(disk) > 0 && disk[len(disk)-1] >= '0' && disk[len(disk)-1] <= '9' {
		prefix += "p"
	}
	suffix := strings.TrimPrefix(name, prefix)
	if suffix == name || suffix == "" {
		return false
	}
	for _, c := range suffix {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func doctorMountFlags() map[string]bool {
	result := map[string]bool{}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return result
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		mount := unescapeMount(fields[1])
		ro := false
		for _, flag := range strings.Split(fields[3], ",") {
			if flag == "ro" {
				ro = true
			}
		}
		result[mount] = ro
	}
	return result
}
func doctorDiskID(name string) string {
	// A real hardware identifier wins; fallback is explicitly non-stable.
	for _, candidate := range []string{"device/wwid", "wwid", "device/serial", "serial"} {
		data, e := os.ReadFile(filepath.Join("/sys/block", name, candidate))
		if e != nil {
			continue
		}
		id := strings.TrimSpace(string(data))
		if id != "" && len(id) < 160 {
			return "hw:" + id
		}
	}
	return "volatile:" + name
}
func doctorSnapshot(now time.Time) doctorView {
	mounts := readAllMounts(nil)
	flags := doctorMountFlags()
	rawDisks := userBlockDevices(readAllBlockDevices(nil))
	result := doctorView{Disks: make([]doctorDisk, 0, len(rawDisks)), Incidents: []doctorIncident{}, HistoryScope: "since_first_storage_doctor_request", CollectedAt: now, SmartctlInstalled: doctorSmartBinary() != ""}
	mountMap := map[string]doctorMount{}
	entwareMount := doctorMount{}
	for _, m := range mounts {
		free, total := doctorFSInodes(m.Mount)
		value := doctorMount{Device: m.Device, Mount: m.Mount, FSType: m.FSType, ReadOnly: flags[m.Mount], TotalBytes: m.TotalBytes, AvailableBytes: m.AvailableBytes, UsedPct: m.UsedPct, InodesFree: free, InodesTotal: total}
		mountMap[m.Mount] = value
		if m.Mount == "/opt" {
			entwareMount = value
			result.Entware.Mounted = true
			result.Entware.Device = m.Device
			result.Entware.FSType = m.FSType
			result.Entware.Writable = !value.ReadOnly
		}
	}
	result.Entware.InitPresent = doctorExistsDir("/opt/etc/init.d")
	result.Entware.PackageDBPresent = doctorExistsFile("/opt/lib/opkg/status") || doctorExistsFile("/opt/var/lib/opkg/status")
	if !result.Entware.Mounted {
		result.Entware.Warning = "Entware mount /opt not found"
	} else if !result.Entware.Writable {
		result.Entware.Warning = "Entware filesystem is read-only"
	} else if !result.Entware.InitPresent {
		result.Entware.Warning = "Entware init directory is missing"
	}
	for _, d := range rawDisks {
		if !doctorDeviceName.MatchString(d.Name) {
			continue
		}
		entry := doctorDisk{ID: doctorDiskID(d.Name), Device: d.Path, Model: d.Model, Vendor: d.Vendor, Transport: d.Transport, SizeBytes: d.SizeBytes, Removable: d.Removable, ReadOnly: d.ReadOnly, Mounts: []doctorMount{}, Partitions: []string{}}
		partitionSet := map[string]bool{}
		for _, m := range mountMap {
			if !doctorParent(d.Name, m.Device) {
				continue
			}
			entry.Mounts = append(entry.Mounts, m)
			if !partitionSet[m.Device] {
				entry.Partitions = append(entry.Partitions, m.Device)
				partitionSet[m.Device] = true
			}
			if m.Mount == "/opt" {
				entry.Entware = true
			}
		}
		sort.Slice(entry.Mounts, func(i, j int) bool { return entry.Mounts[i].Mount < entry.Mounts[j].Mount })
		sort.Strings(entry.Partitions)
		result.Disks = append(result.Disks, entry)
	}
	sort.Slice(result.Disks, func(i, j int) bool {
		if result.Disks[i].Entware != result.Disks[j].Entware {
			return result.Disks[i].Entware
		}
		return result.Disks[i].Device < result.Disks[j].Device
	})
	doctorTrackMounts(now, mountMap)
	doctorTracker.Lock()
	result.Incidents = append(result.Incidents, doctorTracker.events...)
	doctorTracker.Unlock()
	_ = entwareMount
	doctorHistoryAugment(&result, now)
	return result
}
func doctorExistsDir(path string) bool { f, e := os.Stat(path); return e == nil && f.IsDir() }
func doctorExistsFile(path string) bool {
	f, e := os.Stat(path)
	return e == nil && f.Mode().IsRegular()
}
func doctorTrackMounts(now time.Time, current map[string]doctorMount) {
	doctorTracker.Lock()
	defer doctorTracker.Unlock()
	next := map[string]bool{}
	for mount, m := range current {
		if mount != "/opt" && !strings.HasPrefix(mount, "/tmp/mnt/") && !strings.HasPrefix(mount, "/opt/mnt/") {
			continue
		}
		next[mount] = m.ReadOnly
	}
	if doctorTracker.initialized {
		for mount, wasRO := range doctorTracker.mounts {
			isRO, ok := next[mount]
			if !ok {
				doctorAppendIncidentLocked(doctorIncident{At: now, Kind: "disappeared", Mount: mount, Detail: "Previously observed mount is no longer present"})
				continue
			}
			if !wasRO && isRO {
				doctorAppendIncidentLocked(doctorIncident{At: now, Kind: "read_only", Mount: mount, Detail: "Filesystem changed from writable to read-only"})
			}
			if wasRO && !isRO {
				doctorAppendIncidentLocked(doctorIncident{At: now, Kind: "writable_again", Mount: mount, Detail: "Filesystem is writable again"})
			}
		}
		for mount := range next {
			if _, ok := doctorTracker.mounts[mount]; !ok {
				doctorAppendIncidentLocked(doctorIncident{At: now, Kind: "appeared", Mount: mount, Detail: "Mount appeared"})
			}
		}
	}
	doctorTracker.mounts = next
	doctorTracker.initialized = true
}
func doctorAppendIncidentLocked(event doctorIncident) {
	doctorTracker.events = append([]doctorIncident{event}, doctorTracker.events...)
	if len(doctorTracker.events) > doctorMaxIncidents {
		doctorTracker.events = doctorTracker.events[:doctorMaxIncidents]
	}
}
func doctorSmartBinary() string {
	for _, p := range []string{"/opt/sbin/smartctl", "/opt/bin/smartctl", "/usr/sbin/smartctl"} {
		if s, e := os.Stat(p); e == nil && s.Mode().IsRegular() && s.Mode().Perm()&0111 != 0 {
			return p
		}
	}
	return ""
}

type doctorSmartAttr struct {
	ID     int             `json:"id"`
	Name   string          `json:"name"`
	Value  *int            `json:"value,omitempty"`
	Worst  *int            `json:"worst,omitempty"`
	Thresh *int            `json:"thresh,omitempty"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}
type doctorSmartResult struct {
	Device      string            `json:"device"`
	Driver      string            `json:"driver"`
	Status      string            `json:"status"`
	Health      string            `json:"health"`
	Note        string            `json:"note,omitempty"`
	Model       string            `json:"model,omitempty"`
	Temperature *float64          `json:"temperature_c,omitempty"`
	PowerHours  *int64            `json:"power_hours,omitempty"`
	Attributes  []doctorSmartAttr `json:"attributes"`
	NVMe        map[string]any    `json:"nvme,omitempty"`
	CheckedAt   time.Time         `json:"checked_at"`
	Cached      bool              `json:"cached"`
}
type doctorSmartCache struct {
	Result doctorSmartResult
	At     time.Time
}

var doctorSmartDrivers = map[string]bool{"auto": true, "sat": true, "sat,12": true, "scsi": true, "nvme": true, "ata": true, "usbjmicron": true, "usbcypress": true, "usbsunplus": true, "sntjmicron": true, "sntasmedia": true, "sntrealtek": true}

func doctorSmartAllowed(name string) bool {
	if !doctorDeviceName.MatchString(name) {
		return false
	}
	for _, d := range userBlockDevices(readAllBlockDevices(nil)) {
		if d.Name == name {
			return true
		}
	}
	return false
}
func doctorSmartOutput(ctx context.Context, name, driver string) doctorSmartResult {
	res := doctorSmartResult{Device: "/dev/" + name, Driver: driver, Status: "unavailable", Health: "unknown", Attributes: []doctorSmartAttr{}, CheckedAt: time.Now()}
	binary := doctorSmartBinary()
	if binary == "" {
		res.Note = "smartmontools is not installed"
		return res
	}
	if !doctorSmartAllowed(name) {
		res.Status = "error"
		res.Note = "device is not a present supported disk"
		return res
	}
	if !doctorSmartDrivers[driver] {
		res.Status = "error"
		res.Note = "unsupported SMART device type"
		return res
	}
	key := doctorDiskID(name) + "/" + driver
	doctorSmartState.Lock()
	if cached, ok := doctorSmartState.cache[key]; ok && time.Since(cached.At) < 5*time.Minute {
		res = cached.Result
		res.Cached = true
		doctorSmartState.Unlock()
		return res
	}
	if doctorSmartState.active[key] {
		doctorSmartState.Unlock()
		res.Status = "busy"
		res.Note = "SMART check is already running"
		return res
	}
	doctorSmartState.active[key] = true
	doctorSmartState.Unlock()
	defer func() { doctorSmartState.Lock(); delete(doctorSmartState.active, key); doctorSmartState.Unlock() }()
	probe, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	args := []string{"-j", "-a", "-n", "standby"}
	if driver != "auto" {
		args = append(args, "-d", driver)
	}
	args = append(args, "/dev/"+name)
	command, commandErr := safety.CommandContext(probe, binary, args...)
	if commandErr != nil {
		res.Status = "error"
		res.Note = "SMART command rejected by the safety boundary"
		return res
	}
	command.WaitDelay = 400 * time.Millisecond
	var output doctorLimitedBuffer
	output.Limit = 512 * 1024
	command.Stdout = &output
	var stderr doctorLimitedBuffer
	stderr.Limit = 4096
	command.Stderr = &stderr
	err := command.Run()
	if errors.Is(probe.Err(), context.DeadlineExceeded) {
		res.Status = "busy"
		res.Note = "SMART device did not respond within 8 seconds"
		return res
	}
	if output.Truncated {
		res.Status = "error"
		res.Note = "SMART output exceeded safety limit"
		return res
	}
	if output.Len() == 0 {
		res.Status = "unavailable"
		res.Note = "No SMART data returned; the USB bridge may not support passthrough"
		if err != nil {
			res.Note = "SMART information unavailable"
		}
		return res
	}
	var raw struct {
		Smartctl struct {
			ExitStatus int `json:"exit_status"`
			Messages   []struct {
				String string `json:"string"`
			} `json:"messages"`
		} `json:"smartctl"`
		Device struct {
			ModelName string `json:"name"`
		} `json:"device"`
		ModelName   string `json:"model_name"`
		SmartStatus struct {
			Passed *bool `json:"passed"`
		} `json:"smart_status"`
		Temperature struct {
			Current *float64 `json:"current"`
		} `json:"temperature"`
		PowerOnTime struct {
			Hours *int64 `json:"hours"`
		} `json:"power_on_time"`
		ATA struct {
			Table []doctorSmartAttr `json:"table"`
		} `json:"ata_smart_attributes"`
		NVMe map[string]any `json:"nvme_smart_health_information_log"`
	}
	if e := json.Unmarshal(output.Bytes(), &raw); e != nil {
		res.Status = "error"
		res.Note = "SMART output is not valid JSON"
		return res
	}
	res.Model = raw.ModelName
	res.Temperature = raw.Temperature.Current
	res.PowerHours = raw.PowerOnTime.Hours
	res.Attributes = raw.ATA.Table
	if len(res.Attributes) > 80 {
		res.Attributes = res.Attributes[:80]
	}
	if len(raw.NVMe) > 0 {
		res.NVMe = map[string]any{}
		for _, k := range []string{"critical_warning", "percentage_used", "media_errors", "num_err_log_entries", "available_spare", "available_spare_threshold", "unsafe_shutdowns"} {
			if v, ok := raw.NVMe[k]; ok {
				res.NVMe[k] = v
			}
		}
	}
	res.Status = "available"
	if raw.SmartStatus.Passed != nil {
		if *raw.SmartStatus.Passed {
			res.Health = "ok"
		} else {
			res.Health = "critical"
		}
	}
	for _, attr := range res.Attributes {
		if attr.ID != 5 && attr.ID != 197 && attr.ID != 198 {
			continue
		}
		var val struct {
			Value uint64 `json:"value"`
		}
		if json.Unmarshal(attr.Raw, &val) == nil && val.Value > 0 && res.Health != "critical" {
			res.Health = "warning"
		}
	}
	// NVMe health has a different structure and usually no ATA attribute table.
	if len(res.NVMe) > 0 {
		if n, ok := res.NVMe["critical_warning"].(float64); ok {
			if n > 0 {
				res.Health = "critical"
			} else if res.Health == "unknown" {
				res.Health = "ok"
			}
		}
		if n, ok := res.NVMe["media_errors"].(float64); ok && n > 0 && res.Health != "critical" {
			res.Health = "warning"
		}
	}
	if raw.Smartctl.ExitStatus&8 != 0 {
		res.Health = "critical"
	}
	// SMART reports can be non-zero even with useful JSON; never discard it.
	// Exit bit 1 means device open/identity failure; no false green status.
	if raw.Smartctl.ExitStatus&2 != 0 && raw.SmartStatus.Passed == nil && len(res.Attributes) == 0 && len(res.NVMe) == 0 {
		res.Status = "unsupported"
		res.Health = "unknown"
	}
	if raw.Smartctl.ExitStatus&1 != 0 {
		res.Status = "error"
		res.Health = "unknown"
	}
	for _, m := range raw.Smartctl.Messages {
		msg := strings.ToLower(m.String)
		if strings.Contains(msg, "standby") || strings.Contains(msg, "sleep") {
			res.Status = "asleep"
			res.Health = "unknown"
			res.Note = "Disk is in standby; SMART probe was skipped"
			break
		}
	}
	if res.Status == "available" && res.Health == "unknown" {
		res.Note = "SMART health is not reported by this device"
	}
	if res.Status == "available" || res.Status == "unsupported" || res.Status == "asleep" {
		doctorSmartState.Lock()
		now := time.Now()
		for k, v := range doctorSmartState.cache {
			if now.Sub(v.At) > 5*time.Minute {
				delete(doctorSmartState.cache, k)
			}
		}
		if len(doctorSmartState.cache) >= 64 {
			doctorSmartState.cache = map[string]doctorSmartCache{}
		}
		doctorSmartState.cache[key] = doctorSmartCache{Result: res, At: now}
		doctorSmartState.Unlock()
	}
	doctorRememberSMART(res)
	return res
}

type doctorLimitedBuffer struct {
	bytes.Buffer
	Limit     int
	Truncated bool
}

func (b *doctorLimitedBuffer) Write(p []byte) (int, error) {
	available := b.Limit - b.Len()
	if available <= 0 {
		b.Truncated = true
		return len(p), nil
	}
	if len(p) > available {
		b.Truncated = true
		_, _ = b.Buffer.Write(p[:available])
		return len(p), nil
	}
	return b.Buffer.Write(p)
}
func (s *moduleServer) registerStorageDoctor(mux *http.ServeMux) {
	mux.HandleFunc("/v1/doctor", getOnly(func(w http.ResponseWriter, r *http.Request) { writeJSON(w, http.StatusOK, doctorSnapshot(time.Now())) }))
	mux.HandleFunc("/v1/doctor-smart", getOnly(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("device")), "/dev/")
		driver := strings.TrimSpace(r.URL.Query().Get("driver"))
		if driver == "" {
			driver = "auto"
		}
		if !doctorDeviceName.MatchString(name) || !doctorSmartDrivers[driver] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid SMART device or driver"})
			return
		}
		if !doctorSmartAllowed(name) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "device not present"})
			return
		}
		result := doctorSmartOutput(r.Context(), name, driver)
		writeJSON(w, http.StatusOK, result)
	}))
}

// Test-only parsing helper, intentionally constrained to decimal partition suffixes.
func doctorDiskIndex(partition string) int {
	m := regexp.MustCompile(`([0-9]+)$`).FindString(partition)
	n, _ := strconv.Atoi(m)
	return n
}
func doctorSummaryLabel(d doctorDisk) string {
	if d.Entware {
		return "USB / Entware"
	}
	if d.Model != "" {
		return d.Model
	}
	return fmt.Sprint(d.Device)
}
