//go:build linux

package apptransaction

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Exact cooperative C7 templates. Modified init scripts require a separate
// acceptance record and a reviewed implementation; never infer ENABLED from
// an arbitrary shell script. The observed management state is running-only.
type knownBootIntent struct {
	file, blob, marker string
	abi                bool
}

var deviceBootIntent = map[string]knownBootIntent{
	"routerforge-admin":               {"S91routerforge-admin", "b755540ea8b36ed324fc15cac0fa8509d63a3090", "admin", false},
	"routerforge-dns":                 {"S91routerforge-dns", "9e04896a00191444d1a693e5b4f47418eca88653", "dns", true},
	"routerforge-monitoring":          {"S92routerforge-monitoring", "8ab2d0368943565364d070434fbce7b8c07fac63", "monitoring", true},
	"routerforge-network-tools":       {"S97routerforge-network-tools", "ad03cd1ad71353fb43ebf74b6e0b24114f6383f5", "network-tools", true},
	"routerforge-nfqws-manager":       {"S96routerforge-nfqws-manager", "e787c5960332ab60052c20a319c75e8a893ba9ed", "nfqws-manager", true},
	"routerforge-antiscan-manager":    {"S98routerforge-antiscan-manager", "06134f6afd63bbb1705a192ccc4a12afc9740281", "antiscan-manager", true},
	"routerforge-port-access-manager": {"S95routerforge-port-access-manager", "c7e641a278f90e54bd31451c6dacad028e77fa1b", "port-access-manager", true},
	"routerforge-crowdsec-manager":    {"S99routerforge-crowdsec-manager", "9f7b6cd5bba37bbe453b1432c9a97d10bdda6b72", "", false},
}

func gitBootBlob(raw []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(raw))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil))
}

type bootIntentState struct {
	InitMode                       uint32
	Enabled                        bool
	Init, Disabled, ABI, DNSMarker string
}

func readRunningBootIntent(name string) (bootIntentState, error) {
	spec, ok := deviceBootIntent[name]
	if !ok {
		return bootIntentState{}, errors.New("unknown boot intent")
	}
	init, raw, err := readCoreFile(filepath.Join("/opt/etc/init.d", spec.file), 0)
	if err != nil || init == "" || gitBootBlob(raw) != spec.blob {
		return bootIntentState{}, errors.New("init template not accepted")
	}
	info, err := os.Lstat(filepath.Join("/opt/etc/init.d", spec.file))
	if err != nil || !info.Mode().IsRegular() || !coreEvidenceOwned(info, 0) || info.Mode().Perm()&0022 != 0 || info.Mode().Perm()&0111 == 0 {
		return bootIntentState{}, errors.New("init script not executable or unsafe")
	}
	out := bootIntentState{Enabled: true, Init: init, InitMode: uint32(info.Mode().Perm())}
	if spec.marker != "" {
		out.Disabled, err = readDisabledMarker("/opt/etc/routerforge", spec.marker, 0)
		if err != nil {
			return bootIntentState{}, err
		}
		out.Enabled = out.Disabled == ""
	}
	if spec.abi {
		out.ABI, _, err = readCoreFile("/opt/etc/routerforge/module-abi-v1", 0)
		if err != nil || out.ABI == "" {
			return bootIntentState{}, errors.New("ABI boot intent unavailable")
		}
	}
	if name == "routerforge-dns" {
		out.DNSMarker, _, err = readCoreFile("/opt/etc/routerforge/dns.enabled", 0)
		if err != nil {
			return bootIntentState{}, err
		}
	}
	return out, nil
}
func probeRunningBootIntent(ctx context.Context, name, installed string) (LifecycleObservation, error) {
	before, err := readRunningBootIntent(name)
	if err != nil {
		return LifecycleObservation{}, err
	}
	health, err := ProbeRunningModuleHealth(ctx, name, installed)
	if err != nil {
		return LifecycleObservation{}, err
	}
	after, err := readRunningBootIntent(name)
	if err != nil || before != after {
		return LifecycleObservation{}, errors.New("boot intent drift during native probe")
	}
	hash, err := lifecycleIntentHash(before)
	if err != nil {
		return LifecycleObservation{}, err
	}
	return LifecycleObservation{Mode: "running", Enabled: before.Enabled, Ready: true, RuntimeVersion: health.RuntimeVersion, IntentSHA256: hash}, nil
}

// A positively absent modules-disabled directory under a trusted config root
// means no disable markers, not an unknown permission error or unsafe link.
func readDisabledMarker(root, name string, uid uint32) (string, error) {
	if !safeEvidenceDirectory(root, uid, false) {
		return "", errors.New("unsafe disable-marker root")
	}
	dir := filepath.Join(root, "modules-disabled")
	if _, err := os.Lstat(dir); errors.Is(err, os.ErrNotExist) {
		if _, err = os.Lstat(dir); !errors.Is(err, os.ErrNotExist) {
			return "", errors.New("disable-marker directory changed")
		}
		return "", nil
	} else if err != nil {
		return "", err
	}
	hash, _, err := readCoreFile(filepath.Join(dir, name), uid)
	return hash, err
}
