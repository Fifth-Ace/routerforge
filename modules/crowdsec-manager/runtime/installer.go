package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Fifth-Ace/routerforge/internal/safety"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	bouncerRelease    = "v0.0.34"
	bouncerAssetURL   = "https://github.com/crowdsecurity/cs-firewall-bouncer/releases/download/v0.0.34/crowdsec-firewall-bouncer-linux-arm64.tgz"
	bouncerAssetSHA   = "41899de18ad928e89de26a6fcd46ae8c7cb9a3b95369e850335106db0bf727aa"
	bouncerBinaryPath = "/opt/bin/cs-firewall-bouncer"
)

type installerState struct {
	Busy      bool   `json:"busy"`
	Stage     string `json:"stage"`
	Detail    string `json:"detail"`
	Error     string `json:"error,omitempty"`
	Completed bool   `json:"completed"`
	Installed bool   `json:"installed"`
	Platform  string `json:"platform"`
	Supported bool   `json:"supported"`
	Release   string `json:"release"`
}

var installer = struct {
	sync.Mutex
	state installerState
}{}

func installerPlatform() (string, bool) {
	// Go architecture matches the running manager's Entware package target.
	// Do not claim support merely because the RouterForge cross-compiler can build.
	switch runtime.GOARCH {
	case "arm64":
		return "aarch64-3.10", true
	case "mips":
		return "mips-3.4", false
	case "mipsle":
		return "mipsel-3.4", false
	default:
		return runtime.GOARCH, false
	}
}
func installerSnapshot() installerState {
	installer.Lock()
	defer installer.Unlock()
	s := installer.state
	s.Platform, s.Supported = installerPlatform()
	s.Release = bouncerRelease
	s.Installed = installed()
	return s
}
func installerProgress(stage, detail string, err error, completed bool) {
	installer.Lock()
	defer installer.Unlock()
	installer.state.Stage = stage
	installer.state.Detail = detail
	installer.state.Completed = completed
	installer.state.Busy = !completed
	if err != nil {
		installer.state.Error = err.Error()
	}
}
func installerStart(w http.ResponseWriter, r *http.Request) {
	mutation(w, r, func(req map[string]string) error {
		if req["confirm"] != "crowdsec-install" {
			return errors.New("installation confirmation required")
		}
		_, ok := installerPlatform()
		if !ok {
			return errors.New("no verified bouncer build for this Entware architecture")
		}
		if installed() {
			return errors.New("bouncer already installed; existing installation is preserved")
		}
		if err := safeURL(req["api_url"]); err != nil {
			return err
		}
		if !strings.HasPrefix(req["api_url"], "https://") {
			return errors.New("installation requires HTTPS LAPI")
		}
		key := req["api_key"]
		if len(key) < 8 || len(key) > 2048 || strings.ContainsAny(key, "\r\n\x00") {
			return errors.New("valid LAPI API key required")
		}
		installer.Lock()
		if installer.state.Busy {
			installer.Unlock()
			return errors.New("installation already in progress")
		}
		installer.state = installerState{Busy: true, Stage: "queued", Detail: "Preparing verified installation"}
		installer.Unlock()
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
			defer cancel()
			if err := installerExecute(ctx, req["api_url"], key); err != nil {
				installerProgress("failed", "Installation stopped", err, true)
				return
			}
			installerProgress("complete", "Bouncer installed; firewall verification follows", nil, true)
		}()
		return nil
	})
}
func installerExecute(ctx context.Context, url, key string) error {
	installerProgress("dependencies", "Installing ipset and iptables", nil, false)
	// Entware needs these helpers for ipsets and the FORWARD DROP chain.
	if _, err := safety.RunCommand(ctx, 24<<10, "/opt/bin/opkg", "update"); err != nil {
		return errors.New("opkg update failed")
	}
	if _, err := safety.RunCommand(ctx, 24<<10, "/opt/bin/opkg", "install", "ipset", "iptables"); err != nil {
		return errors.New("opkg dependency installation failed")
	}
	installerProgress("download", "Downloading pinned official bouncer archive", nil, false)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, bouncerAssetURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("release download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("release download HTTP %d", resp.StatusCode)
	}
	archive, err := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
	if err != nil {
		return err
	}
	if len(archive) == 0 || len(archive) >= 24<<20 {
		return errors.New("release archive exceeds size limit")
	}
	digest := sha256.Sum256(archive)
	if hex.EncodeToString(digest[:]) != bouncerAssetSHA {
		return errors.New("official release SHA256 mismatch")
	}
	installerProgress("extract", "Extracting verified bouncer binary", nil, false)
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var binary []byte
	for {
		entry, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		name := filepath.Base(entry.Name)
		if entry.Typeflag != tar.TypeReg || (name != "crowdsec-firewall-bouncer" && name != "cs-firewall-bouncer") {
			continue
		}
		if entry.Size <= 0 || entry.Size > 32<<20 {
			return errors.New("invalid bouncer binary size")
		}
		binary, err = io.ReadAll(io.LimitReader(tr, 32<<20))
		if err != nil {
			return err
		}
		break
	}
	if len(binary) < 1000 {
		return errors.New("bouncer executable not found in verified archive")
	}
	if len(binary) < 20 || string(binary[:4]) != "\x7fELF" || binary[4] != 2 || binary[5] != 1 || binary[18] != 183 {
		return errors.New("binary ELF architecture mismatch (ARM64 expected)")
	}
	// Plan all owned paths before modifying any of them; never overwrite existing external files.
	paths := []string{bouncerBinaryPath, bouncerConfigPath, bouncerInitPath}
	for _, p := range paths {
		_, e := os.Lstat(p)
		if e == nil {
			return fmt.Errorf("existing file %s preserved; manual adoption required", p)
		}
		if !os.IsNotExist(e) {
			return fmt.Errorf("cannot inspect %s", p)
		}
	}
	installerProgress("configure", "Writing Entware service and LAPI configuration", nil, false)
	if err := os.MkdirAll("/opt/etc/crowdsec", 0700); err != nil {
		return err
	}
	config := fmt.Sprintf("mode: iptables\nupdate_frequency: 10s\nlog_mode: stdout\nlog_level: info\napi_url: %q\napi_key: %q\ninsecure_skip_verify: false\ndisable_ipv6: true\ndeny_action: DROP\ndeny_log: false\nsupported_decisions_types:\n  - ban\nblacklists_ipv4: crowdsec-blacklists\nblacklists_ipv6: crowdsec6-blacklists\nipset_type: nethash\niptables_chains:\n  - FORWARD\niptables_add_rule_comments: false\nprometheus:\n  enabled: true\n  listen_addr: 127.0.0.1\n  listen_port: 60601\n", url, key)
	service := "#!/bin/sh\nENABLED=yes\nPROCS=cs-firewall-bouncer\nARGS=\"-c /opt/etc/crowdsec/crowdsec-firewall-bouncer.yaml\"\nPREARGS=\"\"\nDESC=\"CrowdSec Firewall Bouncer\"\nPATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin\n. /opt/etc/init.d/rc.func\n"
	// Roll back only files created by this installation, not existing Entware packages.
	created := []string{}
	defer func() {
		if len(created) > 0 {
			for i := len(created) - 1; i >= 0; i-- {
				_ = os.Remove(created[i])
			}
		}
	}()
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{{bouncerConfigPath, []byte(config), 0600}, {bouncerInitPath, []byte(service), 0755}, {bouncerBinaryPath, binary, 0755}} {
		if err := writeExclusive(file.path, file.data, file.mode); err != nil {
			return err
		}
		created = append(created, file.path)
	}
	installerProgress("start", "Starting bouncer using Entware init", nil, false)
	startCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	if _, err := safety.RunCommand(startCtx, 8<<10, bouncerInitPath, "start"); err != nil {
		return errors.New("bouncer start failed; created files rolled back")
	}
	// Service may start asynchronously; don't announce ACTIVE until actual firewall checks pass.
	created = nil
	return nil
}
func writeExclusive(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(path)
		return errors.New("failed to write managed file")
	}
	return nil
}
