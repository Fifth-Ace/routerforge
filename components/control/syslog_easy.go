package main

// P45.4: approachable device aliases, time-window files, and mounted SMB/USB
// discovery. This code never mounts shares and never stores SMB credentials.

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var rfSyslogFriendlyName = regexp.MustCompile(`^[\p{L}\p{N}_-]{1,48}$`)
var rfSyslogDeviceIP = regexp.MustCompile(`^[0-9.]+$`)

func rfSyslogValidateSimple(s rfSyslogSettings) error {
	if s.RotateHours != 0 && s.RotateHours != 1 && s.RotateHours != 6 && s.RotateHours != 12 && s.RotateHours != 24 {
		return errors.New("file interval must be 1, 6, 12 or 24 hours")
	}
	if len(s.DeviceNames) > 8 {
		return errors.New("at most 8 named devices are supported")
	}
	used := map[string]bool{"entware": true}
	for ip, alias := range s.DeviceNames {
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil || parsed.String() != ip || !rfSyslogDeviceIP.MatchString(ip) {
			return errors.New("named device requires canonical IPv4 address")
		}
		if !rfSyslogFriendlyName.MatchString(alias) {
			return errors.New("device name must contain only letters, digits, _ or - (max 48)")
		}
		id := strings.ToLower(alias)
		if used[id] {
			return errors.New("device names must be unique and cannot use entware")
		}
		used[id] = true
	}
	return nil
}
func rfSyslogHours(s rfSyslogSettings) int {
	if s.RotateHours == 0 {
		return 24
	}
	return s.RotateHours
}
func rfSyslogOutputName(s rfSyslogSettings, ip string) string {
	if name := s.DeviceNames[ip]; name != "" {
		return name
	}
	return ip
}

type rfSyslogMountOption struct {
	Root       string `json:"root"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Mounted    bool   `json:"mounted"`
	TotalBytes uint64 `json:"total_bytes,omitempty"`
	FreeBytes  uint64 `json:"free_bytes,omitempty"`
}

type rfSyslogMount struct{ Point, FS, Device string }

func rfSyslogMounted() []rfSyslogMount {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}
	var mounts []rfSyslogMount
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		// /proc/mounts encodes whitespace in octal, never pass through a shell.
		clean := func(x string) string {
			return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(x)
		}
		mounts = append(mounts, rfSyslogMount{Point: clean(f[1]), FS: f[2], Device: clean(f[0])})
	}
	return mounts
}
func rfSyslogIsSMB(fs string) bool { return fs == "cifs" || fs == "smb3" }
func rfSyslogKind(fs string) string {
	if rfSyslogIsSMB(fs) {
		return "smb"
	}
	switch fs {
	case "ext2", "ext3", "ext4", "vfat", "exfat", "fuseblk", "ntfs", "ntfs3", "f2fs", "btrfs", "xfs":
		return "usb"
	}
	return ""
}

// Determine the *physical* Entware device using mount table aliases. Keenetic
// mounts a single USB partition both at /opt and /tmp/mnt/<uuid>. These paths
// must be presented as ONE disk, not as separate storage devices.
func rfSyslogEntwareUSB(mounts []rfSyslogMount) (string, bool) {
	var opt rfSyslogMount
	for _, m := range mounts {
		if m.Point == "/opt" {
			opt = m
			break
		}
	}
	if opt.Device == "" || rfSyslogKind(opt.FS) != "usb" {
		return "", false
	}
	for _, m := range mounts {
		if strings.HasPrefix(m.Point, "/tmp/mnt/") && filepath.Dir(m.Point) == "/tmp/mnt" && m.Device == opt.Device && m.FS == opt.FS {
			return opt.Device, true
		}
	}
	return opt.Device, false
}
func rfSyslogStorageOptions() []rfSyslogMountOption {
	mounts := rfSyslogMounted()
	var out []rfSyslogMountOption
	seenRoot := map[string]bool{}
	seenDevice := map[string]bool{}
	mountSizes := map[string]storageInfo{}
	for _, disk := range readStorage() {
		mountSizes[disk.Mount] = disk
	}
	newOption := func(root, label, kind, point string) rfSyslogMountOption {
		opt := rfSyslogMountOption{Root: root, Label: label, Kind: kind, Mounted: true}
		if disk, ok := mountSizes[point]; ok {
			opt.TotalBytes = disk.TotalBytes
			opt.FreeBytes = disk.FreeBytes
		}
		return opt
	}
	device, usb := rfSyslogEntwareUSB(mounts)
	if usb {
		out = append(out, newOption(rfSyslogDefaultRoot, "USB с Entware ("+device+")", "usb", "/opt"))
		seenRoot[rfSyslogDefaultRoot] = true
		seenDevice[device] = true
	} else {
		// A writable Entware mount is not necessarily a USB disk (NAND, SATA).
		for _, m := range mounts {
			if m.Point == "/opt" && rfSyslogKind(m.FS) == "usb" {
				out = append(out, newOption(rfSyslogDefaultRoot, "Entware (/opt)", "local", "/opt"))
				seenRoot[rfSyslogDefaultRoot] = true
				break
			}
		}
	}
	for _, m := range mounts {
		kind := rfSyslogKind(m.FS)
		if kind == "" {
			continue
		}
		if seenDevice[m.Device] && !rfSyslogIsSMB(m.FS) {
			continue
		}
		// Existing direct mount under /opt/mnt and other Keenetic USB volumes
		// mounted at /tmp/mnt/<uuid> are supported, but never guessed from a
		// plain directory on the /opt filesystem.
		if !(filepath.Dir(m.Point) == "/opt/mnt" || filepath.Dir(m.Point) == "/tmp/mnt") {
			continue
		}
		root := filepath.Join(m.Point, "routerforge-syslog")
		if !rfSyslogRootAllowed(root) || seenRoot[root] {
			continue
		}
		label := filepath.Base(m.Point) + " (" + strings.ToUpper(kind) + ")"
		if kind == "smb" {
			label = "NAS / SMB · " + filepath.Base(m.Point)
		}
		out = append(out, newOption(root, label, kind, m.Point))
		seenRoot[root] = true
		if kind == "usb" {
			seenDevice[m.Device] = true
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out
}

func rfSyslogStorageCheck(root string) (bool, string) {
	if !rfSyslogRootAllowed(root) {
		return false, "unsafe log storage path"
	}
	if st, err := os.Lstat(root); err == nil && (!st.IsDir() || st.Mode()&os.ModeSymlink != 0) {
		return false, "log root must be a real directory"
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, "log directory unavailable"
	}
	parent := filepath.Dir(root)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return false, "selected storage not mounted"
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return false, "selected storage unavailable"
	}
	for _, opt := range rfSyslogStorageOptions() {
		if opt.Root == root {
			return true, ""
		}
	}
	// A non-removable /opt/var/log root is allowed only if /opt exists as
	// a persistent mount. An unmounted SMB/USB target is NEVER a fallback.
	return false, fmt.Sprintf("storage is not an active Entware/USB/SMB mount: %s", root)
}

type rfSyslogSourceDetail struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func rfSyslogSourceDetails(parent []byte) []rfSyslogSourceDetail {
	clean := rfSyslogStripComments(string(parent))
	// Lexical identification, NOT a complete syslog-ng parser. An ambiguous
	// source is marked unknown and cannot be used by the simplified wizard.
	re := regexp.MustCompile(`(?s)\bsource\s+([A-Za-z_][A-Za-z_0-9]*)\s*\{([^{}]*)\}`)
	matches := re.FindAllStringSubmatch(clean, -1)
	out := make([]rfSyslogSourceDetail, 0, len(matches))
	for _, m := range matches {
		body := m[2]
		kind := "unknown"
		remote := regexp.MustCompile(`\b(network|udp|tcp|syslog|default-network-drivers)\s*\(`).MatchString(body)
		local := regexp.MustCompile(`\b(internal|unix-dgram|unix-stream)\s*\(`).MatchString(body)
		kernel := strings.Contains(body, "/proc/kmsg") || strings.Contains(body, "/dev/kmsg")
		if remote && !local && !kernel {
			kind = "network"
		}
		if local && !remote && !kernel {
			kind = "local"
		}
		if kernel && !remote && !local {
			kind = "kernel"
		}
		out = append(out, rfSyslogSourceDetail{Name: m[1], Kind: kind})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func rfSyslogCheckSource(s rfSyslogSettings, parent []byte) error {
	types := map[string]string{}
	for _, info := range rfSyslogSourceDetails(parent) {
		types[info.Name] = info.Kind
	}
	if (s.UDP || s.TCP) && s.RemoteSource != "" && types[s.RemoteSource] != "network" {
		return errors.New("selected source is not a verified network listener; choose a network source or create a new listener")
	}
	if s.Local && types[s.LocalSource] != "local" {
		return errors.New("selected local source is not verified to receive local events")
	}
	return nil
}

// rfSyslogSnippetEasy routes one message into exactly one device/file per
// selected receive-time bucket. No shell scripts or additional daemons.
func rfSyslogSnippetEasy(s rfSyslogSettings) string {
	hours := rfSyslogHours(s)
	var b strings.Builder
	b.WriteString(rfSyslogSidecarMarker + "\n")
	source := s.RemoteSource
	if (s.UDP || s.TCP) && source == "" {
		source = "s_routerforge_net"
		b.WriteString("source s_routerforge_net {\n")
		if s.UDP {
			fmt.Fprintf(&b, "  network(ip(\"%s\") port(%d) transport(\"udp\"));\n", s.Bind, s.Port)
		}
		if s.TCP {
			fmt.Fprintf(&b, "  network(ip(\"%s\") port(%d) transport(\"tcp\"));\n", s.Bind, s.Port)
		}
		b.WriteString("};\n")
	}
	// All filenames are deterministic; /path is a validated dedicated root.
	names := make([]string, 0, len(s.DeviceNames))
	for ip := range s.DeviceNames {
		names = append(names, ip)
	}
	sort.Strings(names)
	for i, ip := range names {
		// SourceIP is a numeric IPv4 address. regexp.QuoteMeta avoids dots as wildcards.
		fmt.Fprintf(&b, "filter f_routerforge_device_%d { match(\"^%s$\" value(\"SOURCEIP\")); };\n", i, regexp.QuoteMeta(ip))
	}
	b.WriteString("filter f_routerforge_allow { ")
	for i, cidr := range s.Allowed {
		if i > 0 {
			b.WriteString(" or ")
		}
		fmt.Fprintf(&b, "netmask(\"%s\")", cidr)
	}
	b.WriteString("; };\n")
	for bucket := 0; bucket < 24; bucket += hours {
		if hours == 24 {
			break
		}
		pieces := make([]string, 0, hours)
		for hour := bucket; hour < bucket+hours; hour++ {
			pieces = append(pieces, fmt.Sprintf("%02d", hour))
		}
		fmt.Fprintf(&b, "filter f_routerforge_hour_%02d { match(\"^(%s)$\" value(\"R_HOUR\")); };\n", bucket, strings.Join(pieces, "|"))
	}
	emit := func(category, name, logSource string, filters []string) {
		// logSource is always a preflight-validated source identifier.
		for bucket := 0; bucket < 24; bucket += hours {
			id := fmt.Sprintf("%s_%02d", category, bucket)
			filename := "syslog-" + name
			if hours != 24 {
				filename += fmt.Sprintf("-%02d-%02d", bucket, bucket+hours)
			}
			filename += ".log"
			fmt.Fprintf(&b, "destination d_routerforge_%s { file(\"%s/${R_YEAR}/${R_MONTH}/${R_DAY}/%s\" perm(0640) create_dirs(no)); };\n", id, s.Root, filename)
			fmt.Fprintf(&b, "log { source(%s); ", logSource)
			for _, f := range filters {
				fmt.Fprintf(&b, "filter(%s); ", f)
			}
			if hours != 24 {
				fmt.Fprintf(&b, "filter(f_routerforge_hour_%02d); ", bucket)
			}
			fmt.Fprintf(&b, "destination(d_routerforge_%s); };\n", id)
		}
	}
	if s.UDP || s.TCP {
		for i, ip := range names {
			emit(fmt.Sprintf("named_%d", i), s.DeviceNames[ip], source, []string{"f_routerforge_allow", fmt.Sprintf("f_routerforge_device_%d", i)})
		}
		if len(names) > 0 {
			b.WriteString("filter f_routerforge_other { ")
			for i := range names {
				if i > 0 {
					b.WriteString(" and ")
				}
				fmt.Fprintf(&b, "not filter(f_routerforge_device_%d)", i)
			}
			b.WriteString("; };\n")
		}
		// For unnamed devices ${SOURCEIP} is resolved by syslog-ng at runtime.
		filters := []string{"f_routerforge_allow"}
		if len(names) > 0 {
			filters = append(filters, "f_routerforge_other")
		}
		emit("other", "${SOURCEIP}", source, filters)
	}
	if s.Local {
		b.WriteString("filter f_routerforge_entware { ")
		for i, prog := range rfSyslogEntwarePrograms {
			if i > 0 {
				b.WriteString(" or ")
			}
			fmt.Fprintf(&b, "program(\"%s\")", prog)
		}
		b.WriteString("; };\nfilter f_routerforge_router { not filter(f_routerforge_entware); };\n")
		emit("local_entware", "entware", s.LocalSource, []string{"f_routerforge_entware"})
		emit("local_router", rfSyslogOutputName(s, s.HeadIP), s.LocalSource, []string{"f_routerforge_router"})
	}
	return b.String()
}

// Date directories are created only while storage is positively mounted.
// syslog-ng destinations use create_dirs(no); after a disk/share disappears
// it must NOT recreate the missing mount tree on the router's flash.
func rfSyslogPrepareFolders(root string, now time.Time) error {
	if ready, why := rfSyslogStorageReady(root); !ready {
		return errors.New(why)
	}
	create := func(p string) error {
		if err := os.Mkdir(p, 0750); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		st, err := os.Lstat(p)
		if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errors.New("log directory is not a real directory")
		}
		return nil
	}
	if err := create(root); err != nil {
		return err
	}
	for _, d := range []time.Time{now, now.AddDate(0, 0, 1)} {
		cur := root
		for _, part := range []string{d.Format("2006"), d.Format("01"), d.Format("02")} {
			cur = filepath.Join(cur, part)
			if err := create(cur); err != nil {
				return err
			}
		}
	}
	return nil
}

func rfSyslogDisplaySource(source string, hours int) (string, string) {
	if hours >= 24 {
		return source, ""
	}
	parts := strings.Split(source, "-")
	if len(parts) < 3 {
		return source, ""
	}
	a, e1 := strconv.Atoi(parts[len(parts)-2])
	b, e2 := strconv.Atoi(parts[len(parts)-1])
	if e1 != nil || e2 != nil || a < 0 || b > 24 || a >= 24 || b-a != hours || a%hours != 0 {
		return source, ""
	}
	return strings.Join(parts[:len(parts)-2], "-"), fmt.Sprintf("%02d–%02d", a, b)
}
