//go:build linux

package apptransaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const profilingMarkerPath = "/opt/etc/routerforge/profiling.enabled"

type profilingMigrationRecord struct {
	Package      string `json:"package"`
	From         string `json:"from"`
	To           string `json:"to"`
	AssetSHA256  string `json:"asset_sha256"`
	MarkerSHA256 string `json:"marker_sha256"`
	Intent       string `json:"intent"`
}

// Read immediately before the verified OPKG install. A previous package's prerm
// may remove the marker; the incoming payload may create it. The new postinst
// must reconstruct the explicitly recorded intent before restarting Core.
// Only an absent or canonical empty marker is supported. Never guess or
// overwrite a custom marker. Durable evidence is exclusive and never replayed.
func recordProfilingMigration(store *Store, p Package, current, marker string, uid uint32) (string, error) {
	if store == nil || p.Name != "routerforge-profiling" || !digest.MatchString(p.SHA256) || current == p.Version {
		return "", errors.New("invalid profiling migration request")
	}
	if err := allowUpgrade(current, p.Version); err != nil {
		return "", err
	}
	hash, raw, err := readCoreFile(marker, uid)
	if err != nil {
		return "", err
	}
	intent := "off"
	if hash != "" {
		if len(raw) != 0 {
			return "", errors.New("custom profiling marker is unsupported")
		}
		intent = "on"
	}
	b, err := json.Marshal(profilingMigrationRecord{p.Name, current, p.Version, p.SHA256, hash, intent})
	if err != nil {
		return "", err
	}
	if err = durableExclusive(store, "profiling-marker-before.json", b); err != nil {
		return "", err
	}
	again, _, err := readCoreFile(marker, uid)
	if err != nil || again != hash {
		return "", errors.New("profiling marker changed before OPKG")
	}
	return intent, nil
}

// Look for any other process of the lease owner's UID that still references
// the lock inode after OPKG exits. This is a conservative observation, not a
// kernel-enforced proof of absence or an excuse to unlock/kill a holder. An
// unreadable live same-UID process is ambiguous and stops the verified worker.
// Root/external shell and noncooperating processes remain outside the contract.
func foreignLeaseHolder(proc string, lease *os.File, uid uint32, self int) (int, error) {
	if lease == nil {
		return 0, errors.New("missing lease")
	}
	lock, err := lease.Stat()
	if err != nil || !privateRegular(lock) {
		return 0, errors.New("invalid lease inode")
	}
	entries, err := os.ReadDir(proc)
	if err != nil {
		return 0, err
	}
	inspected := 0
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || pid <= 0 || pid == self || !entry.IsDir() {
			continue
		}
		path := filepath.Join(proc, entry.Name())
		info, e := os.Stat(path)
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return 0, e
		}
		owner, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return 0, errors.New("unknown process owner")
		}
		if owner.Uid != uid {
			continue
		}
		fds, e := os.ReadDir(filepath.Join(path, "fd"))
		if errors.Is(e, os.ErrNotExist) {
			continue
		}
		if e != nil {
			return 0, fmt.Errorf("cannot inspect PID %d descriptors: %w", pid, e)
		}
		for _, fd := range fds {
			inspected++
			if inspected > 65536 {
				return 0, errors.New("descriptor observation limit exceeded")
			}
			st, e := os.Stat(filepath.Join(path, "fd", fd.Name()))
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return 0, fmt.Errorf("cannot inspect PID %d descriptor: %w", pid, e)
			}
			if os.SameFile(lock, st) {
				return pid, nil
			}
		}
	}
	return 0, nil
}
