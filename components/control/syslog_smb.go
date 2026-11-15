package main

// P45.5: opt-in SMB mount assistant. Credentials are never passed via shell,
// argv, API read endpoints or logs; an explicit root-session POST is required.
// No software installation or syslog-ng restart is performed implicitly.

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	rfSMBPoint    = "/opt/mnt/rf-syslog-smb"
	rfSMBRoot     = rfSMBPoint + "/routerforge-syslog"
	rfSMBDir      = "/opt/etc/routerforge/syslog-smb"
	rfSMBSecret   = rfSMBDir + "/credentials"
	rfSMBSettings = rfSMBDir + "/connection.json"
)

var rfSMBMu sync.Mutex
var rfSMBHostRE = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,190}$`)
var rfSMBShareRE = regexp.MustCompile(`^[\p{L}\p{N}_.$ -]{1,80}$`)

type rfSMBConnectRequest struct {
	Host     string `json:"host"`
	Share    string `json:"share"`
	Username string `json:"username"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
	Remember bool   `json:"remember"`
	Confirm  string `json:"confirm"`
}
type rfSMBConnection struct {
	Host     string `json:"host"`
	Share    string `json:"share"`
	Remember bool   `json:"remember"`
}

func rfSMBNormalize(host, share string) (string, string, error) {
	// Accept both //HOST/SHARE and \\HOST\SHARE from the UI.
	host = strings.ReplaceAll(strings.TrimSpace(host), `\`, `/`)
	if strings.HasPrefix(host, "//") {
		parts := strings.Split(strings.TrimPrefix(host, "//"), "/")
		if len(parts) != 2 || share != "" {
			return "", "", errors.New("enter a UNC path or host and share, not both")
		}
		host, share = parts[0], parts[1]
	}
	if !rfSMBHostRE.MatchString(host) || strings.Contains(host, "..") || strings.HasSuffix(host, ".") || !rfSMBShareRE.MatchString(share) || strings.Contains(share, "..") {
		return "", "", errors.New("invalid SMB server or share")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback()) {
		return "", "", errors.New("invalid SMB server address")
	}
	return host, share, nil
}
func rfSMBCheckCredentials(req rfSMBConnectRequest) error {
	if req.Confirm != "CONNECT SYSLOG SMB" {
		return errors.New("explicit SMB confirmation required")
	}
	if len(req.Username) == 0 || len(req.Username) > 128 || len(req.Password) == 0 || len(req.Password) > 256 || len(req.Domain) > 64 {
		return errors.New("username, password or domain invalid")
	}
	for _, value := range []string{req.Username, req.Password, req.Domain} {
		if strings.ContainsAny(value, "\x00\r\n") {
			return errors.New("credentials contain invalid control characters")
		}
	}
	return nil
}
func rfSMBMounted() bool {
	for _, m := range rfSyslogMounted() {
		if m.Point == rfSMBPoint && rfSyslogIsSMB(m.FS) {
			return true
		}
	}
	return false
}
func rfSMBMountClient() string {
	for _, p := range []string{"/opt/sbin/mount.cifs", "/opt/bin/mount.cifs", "/sbin/mount.cifs", "/usr/sbin/mount.cifs"} {
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			return p
		}
	}
	return ""
}
func rfSMBUmountBinary() string {
	for _, p := range []string{"/bin/umount", "/opt/bin/umount", "/sbin/umount", "/usr/bin/umount"} {
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			return p
		}
	}
	return ""
}
func rfSMBKernelAvailable() bool {
	if _, e := os.Stat("/sys/module/cifs"); e == nil {
		return true
	}
	data, e := os.ReadFile("/proc/filesystems")
	if e != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) > 0 && f[len(f)-1] == "cifs" {
			return true
		}
	}
	return false
}
func rfSMBReadConnection() rfSMBConnection {
	raw, e := os.ReadFile(rfSMBSettings)
	if e != nil || len(raw) > 4096 {
		return rfSMBConnection{}
	}
	var c rfSMBConnection
	if json.Unmarshal(raw, &c) != nil {
		return rfSMBConnection{}
	}
	return c
}
func rfSyslogSMBStatus(w http.ResponseWriter, r *http.Request) {
	c := rfSMBReadConnection()
	writeJSON(w, 200, map[string]any{"client_ready": rfSMBMountClient() != "", "kernel_ready": rfSMBKernelAvailable(), "connected": rfSMBMounted(), "mount_root": rfSMBRoot, "host": c.Host, "share": c.Share, "auto_reconnect": c.Remember, "credentials_saved": c.Remember && rfSMBSecretFileSecure(), "mutation_api": true})
}
func rfSMBSecretFileSecure() bool {
	st, e := os.Lstat(rfSMBSecret)
	return e == nil && st.Mode().IsRegular() && st.Mode().Perm() == 0600
}
func rfSMBEnsureDir() error {
	if err := os.MkdirAll(rfSMBDir, 0700); err != nil {
		return err
	}
	st, e := os.Lstat(rfSMBDir)
	if e != nil || !st.IsDir() || st.Mode().Perm() != 0700 || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe SMB credential directory")
	}
	return nil
}
func rfSMBWriteTemp(data []byte) (string, error) {
	f, e := os.CreateTemp(rfSMBDir, ".rf-smb-credential-*")
	if e != nil {
		return "", e
	}
	name := f.Name()
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		os.Remove(name)
		return "", e
	}
	return name, nil
}
func rfSMBSafeMountpoint() error {
	if err := os.MkdirAll("/opt/mnt", 0700); err != nil {
		return err
	}
	if st, e := os.Lstat("/opt/mnt"); e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe Entware mount directory")
	}
	if err := os.Mkdir(rfSMBPoint, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	st, e := os.Lstat(rfSMBPoint)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe SMB mountpoint")
	}
	// Never mount over foreign data: this is a dedicated RouterForge mountpoint.
	if !rfSMBMounted() {
		f, e := os.ReadDir(rfSMBPoint)
		if e != nil {
			return e
		}
		if len(f) > 0 {
			return errors.New("mountpoint is not empty; refusing to hide data")
		}
		for _, m := range rfSyslogMounted() {
			if m.Point == rfSMBPoint {
				return errors.New("mountpoint is already used by another filesystem")
			}
		}
	}
	return nil
}
func rfSMBMount(ctx context.Context, host, share, credentialFile string) error {
	if rfSMBMounted() {
		return nil
	}
	if !rfSMBKernelAvailable() || rfSMBMountClient() == "" {
		return errors.New("SMB client or kernel CIFS support missing")
	}
	if err := rfSMBSafeMountpoint(); err != nil {
		return err
	}
	// SMB3 only. A credential-file option avoids passwords in process argv.
	_, e := safety.RunCommand(ctx, 2048, rfSMBMountClient(), "//"+host+"/"+share, rfSMBPoint, "-o", "credentials="+credentialFile+",vers=3.0,nosuid,nodev,noexec,uid=0,gid=0,file_mode=0600,dir_mode=0700")
	if e != nil || !rfSMBMounted() {
		return errors.New("SMB mount failed; verify NAS address, permissions and SMB3 support")
	}
	return nil
}
func rfSMBTestWrite() error {
	if !rfSMBMounted() {
		return errors.New("SMB share is not mounted")
	}
	if err := os.MkdirAll(rfSMBRoot, 0700); err != nil {
		return errors.New("cannot create journal folder on SMB share")
	}
	f, e := os.CreateTemp(rfSMBRoot, ".rf-write-check-*")
	if e != nil {
		return errors.New("SMB share is not writable")
	}
	name := f.Name()
	_, e = f.Write([]byte("RouterForge SMB writable check\n"))
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	_ = os.Remove(name)
	if e != nil {
		return errors.New("SMB write test failed")
	}
	return nil
}
func rfSyslogSMBConnect(w http.ResponseWriter, r *http.Request) {
	var req rfSMBConnectRequest
	if e := decodeMutationJSON(w, r, &req); e != nil {
		writeJSON(w, 400, map[string]any{"error": "invalid SMB request"})
		return
	}
	host, share, e := rfSMBNormalize(req.Host, req.Share)
	if e == nil {
		e = rfSMBCheckCredentials(req)
	}
	if e != nil {
		writeJSON(w, 400, map[string]any{"error": e.Error()})
		return
	}
	rfSMBMu.Lock()
	defer rfSMBMu.Unlock()
	if rfSMBMounted() {
		writeJSON(w, 409, map[string]any{"error": "an SMB share is already connected; disconnect it first"})
		return
	}
	if !rfSMBKernelAvailable() || rfSMBMountClient() == "" {
		writeJSON(w, 409, map[string]any{"error": "SMB client or CIFS kernel support not available. Choose USB or install compatible CIFS support first"})
		return
	}
	if e = rfSMBEnsureDir(); e != nil {
		writeJSON(w, 409, map[string]any{"error": "SMB credentials directory is unsafe"})
		return
	}
	secret := []byte("username=" + req.Username + "\npassword=" + req.Password + "\n")
	if req.Domain != "" {
		secret = append(secret, []byte("domain="+req.Domain+"\n")...)
	}
	temp, e := rfSMBWriteTemp(secret)
	for i := range secret {
		secret[i] = 0
	}
	if e != nil {
		writeJSON(w, 500, map[string]any{"error": "cannot prepare protected SMB credentials"})
		return
	}
	defer os.Remove(temp)
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if e = rfSMBMount(ctx, host, share, temp); e != nil {
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	if e = rfSMBTestWrite(); e != nil {
		if path := rfSMBUmountBinary(); path != "" {
			_, _ = safety.RunCommand(ctx, 2048, path, rfSMBPoint)
		}
		writeJSON(w, 409, map[string]any{"error": e.Error()})
		return
	}
	if req.Remember {
		if e = os.Rename(temp, rfSMBSecret); e != nil {
			writeJSON(w, 500, map[string]any{"error": "SMB connected but credentials could not be saved for reconnect"})
			return
		}
		encoded, _ := json.Marshal(rfSMBConnection{Host: host, Share: share, Remember: true})
		if e = rfSyslogAtomic(rfSMBSettings, encoded); e != nil {
			_ = os.Remove(rfSMBSecret)
			writeJSON(w, 500, map[string]any{"error": "SMB connected but auto reconnect settings could not be saved"})
			return
		}
	} else {
		_ = os.Remove(rfSMBSettings)
		_ = os.Remove(rfSMBSecret)
	}
	writeJSON(w, 200, map[string]any{"ok": true, "mounted": true, "root": rfSMBRoot, "auto_reconnect": req.Remember})
}
func rfSyslogSMBDisconnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	if e := decodeMutationJSON(w, r, &req); e != nil || req.Confirm != "DISCONNECT SYSLOG SMB" {
		writeJSON(w, 409, map[string]any{"error": "explicit disconnect confirmation required"})
		return
	}
	rfSMBMu.Lock()
	defer rfSMBMu.Unlock()
	if rfSMBMounted() {
		s, e := rfSyslogStored()
		if e == nil && s.Root == rfSMBRoot {
			_, running := rfSyslogService()
			if running {
				writeJSON(w, 409, map[string]any{"error": "stop syslog-ng before disconnecting its active SMB storage"})
				return
			}
		}
		path := rfSMBUmountBinary()
		if path == "" {
			writeJSON(w, 409, map[string]any{"error": "umount utility missing"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		_, e = safety.RunCommand(ctx, 2048, path, rfSMBPoint)
		if e != nil || rfSMBMounted() {
			writeJSON(w, 409, map[string]any{"error": "SMB share is busy or could not be unmounted"})
			return
		}
	}
	_ = os.Remove(rfSMBSettings)
	_ = os.Remove(rfSMBSecret)
	writeJSON(w, 200, map[string]any{"ok": true, "mounted": false})
}

// Deliberately serialized and opt-in: the saved 0600 credentials are necessary
// for reconnect after reboot. Never unmount a busy share or restart syslog-ng.
func rfSyslogSMBReconnectLoop() {
	ticker := time.NewTicker(75 * time.Second)
	defer ticker.Stop()
	for {
		rfSyslogSMBReconnectOnce()
		<-ticker.C
	}
}
func rfSyslogSMBReconnectOnce() {
	rfSMBMu.Lock()
	defer rfSMBMu.Unlock()
	if rfSMBMounted() || !rfSMBSecretFileSecure() {
		return
	}
	c := rfSMBReadConnection()
	if !c.Remember {
		return
	}
	host, share, e := rfSMBNormalize(c.Host, c.Share)
	if e != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
	defer cancel()
	if rfSMBMount(ctx, host, share, rfSMBSecret) == nil {
		if rfSMBTestWrite() == nil {
			if settings, err := rfSyslogStored(); err == nil && settings.Root == rfSMBRoot {
				_ = rfSyslogPrepareFolders(rfSMBRoot, time.Now())
			}
		}
	}
}
