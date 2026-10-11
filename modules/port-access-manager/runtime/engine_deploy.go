package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const engineBase = "/opt/etc/routerforge/port-access-manager"
const engineScript = "/opt/share/routerforge/modules/port-access-manager/engine-gate.sh"
const engineNDMHook = "/opt/etc/ndm/netfilter.d/101routerforge-access.sh"
const engineNDMContent = "#!/bin/sh\n[ \"${type:-iptables}\" = ip6tables ] && exit 0\n[ \"${table:-filter}\" = filter ] || exit 0\nfor engine in knockd fwknopd; do\n  [ -f /opt/etc/routerforge/port-access-manager/$engine.enabled ] || continue\n  /bin/sh /opt/share/routerforge/modules/port-access-manager/engine-gate.sh apply \"$engine\" || :\ndone\nexit 0\n"

var engineDeployMu sync.Mutex

type engineDeployment struct {
	Engine          string              `json:"engine"`
	Action          string              `json:"action"`
	Confirm         string              `json:"confirm"`
	Wizard          EngineWizardRequest `json:"wizard"`
	ReplaceExisting bool                `json:"replace_existing"`
}

func engineFiles(engine string) []string {
	if engine == "knockd" {
		return []string{"/opt/etc/knockd.conf"}
	}
	if engine == "fwknopd" {
		return []string{"/opt/etc/fwknop/access.conf", "/opt/etc/fwknop/fwknopd.conf"}
	}
	return nil
}
func engineActive(engine string) bool {
	_, e := os.Stat(filepath.Join(engineBase, engine+".enabled"))
	return e == nil
}
func engineGateConfig(req EngineWizardRequest) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "WAN=%s\nTTL=%d\n", req.Interface, req.AccessSeconds)
	for _, s := range req.Services {
		fmt.Fprintf(&b, "SERVICE=%s:%d\n", s.Address, s.Port)
	}
	return []byte(b.String())
}
func generatedConfig(req EngineWizardRequest) (map[string][]byte, string, error) {
	if e := validateWizard(req); e != nil {
		return nil, "", e
	}
	files := map[string][]byte{}
	if req.Engine == "knockd" {
		files["/opt/etc/knockd.conf"] = []byte(fmt.Sprintf("[options]\nInterface = %s\nUseSyslog\n\n[routerforge-access]\nsequence = %d,%d,%d\nseq_timeout = %d\ntcpflags = syn\nStart_Command = /bin/sh %s authorize knockd %%IP%%\n", req.Interface, req.Knock[0], req.Knock[1], req.Knock[2], req.SequenceSeconds, engineScript))
		return files, "", nil
	}
	key, mac, e := generateSPAKeys()
	if e != nil {
		return nil, "", e
	}
	var ports []string
	for _, s := range req.Services {
		ports = append(ports, "tcp/"+strconv.Itoa(s.Port))
	}
	files["/opt/etc/fwknop/access.conf"] = []byte("SOURCE ANY;\nOPEN_PORTS " + strings.Join(ports, ",") + ";\nKEY_BASE64 " + key + ";\nHMAC_KEY_BASE64 " + mac + ";\nFW_ACCESS_TIMEOUT " + strconv.Itoa(req.AccessSeconds) + ";\n")
	files["/opt/etc/fwknop/fwknopd.conf"] = []byte(fmt.Sprintf("PCAP_INTF %s;\nENABLE_IPT_FORWARDING Y;\nIPT_FORWARD_ACCESS ACCEPT,filter,RFPA_F_GATE,1,FWKNOP_FORWARD,1;\nENABLE_IPT_LOCAL_NAT N;\nFLUSH_IPT_AT_INIT Y;\nFLUSH_IPT_AT_EXIT Y;\n", req.Interface))
	return files, "ENCRYPTION_KEY_BASE64=" + key + "\nHMAC_KEY_BASE64=" + mac, nil
}
func engineRequestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		jsonReply(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	engineDeployMu.Lock()
	defer engineDeployMu.Unlock()
	if r.Method == http.MethodGet {
		engine := r.URL.Query().Get("engine")
		if len(engineFiles(engine)) == 0 {
			jsonReply(w, 400, map[string]string{"error": "unknown engine"})
			return
		}
		jsonReply(w, 200, map[string]any{"engine": engine, "prepared": fileExists(filepath.Join(engineBase, engine+".gate")), "enabled": engineActive(engine), "confirmed": engineIsConfirmed(engine), "running": serviceSnapshot(engine, "/opt/etc/init.d").Running, "firewall_policy": "preserve"})
		return
	}
	if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
		jsonReply(w, 403, map[string]string{"error": "Core authorization required"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	var req engineDeployment
	if e := d.Decode(&req); e != nil {
		jsonReply(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		jsonReply(w, 400, map[string]string{"error": "trailing request"})
		return
	}
	if req.Confirm != "APPLY" || len(engineFiles(req.Engine)) == 0 {
		jsonReply(w, 400, map[string]string{"error": "invalid engine or confirmation"})
		return
	}
	var err error
	var credential string
	switch req.Action {
	case "prepare":
		credential, err = prepareEngine(req.Engine, req.Wizard, req.ReplaceExisting)
	case "enable":
		err = enableEngine(req.Engine)
	case "confirm":
		err = confirmEngine(req.Engine)
	case "disable":
		err = disableEngine(req.Engine)
	default:
		jsonReply(w, 400, map[string]string{"error": "invalid action"})
		return
	}
	if err != nil {
		jsonReply(w, 409, map[string]string{"error": err.Error()})
		return
	}
	// Secrets are never available via GET or status; only the explicit preparation response.
	jsonReply(w, 200, map[string]any{"ok": true, "engine": req.Engine, "action": req.Action, "enabled": engineActive(req.Engine), "credential_once": credential})
}
func fileExists(path string) bool { s, e := os.Lstat(path); return e == nil && s.Mode().IsRegular() }
func engineIsConfirmed(engine string) bool {
	b, e := os.ReadFile(filepath.Join(engineBase, engine+".enabled"))
	if e != nil {
		return false
	}
	token := strings.TrimSpace(string(b))
	return validEngineTransaction(token) && fileExists(filepath.Join(engineBase, "transactions", token, "confirmed"))
}
func prepareEngine(engine string, req EngineWizardRequest, replace bool) (string, error) {
	if req.Engine != engine {
		return "", errors.New("engine mismatch")
	}
	if engineActive(engine) || engineActive(otherEngine(engine)) {
		return "", errors.New("disable active managed gate before editing")
	}
	service := serviceSnapshot(engine, "/opt/etc/init.d")
	if !service.Installed {
		return "", errors.New("install upstream daemon first")
	}
	if service.Running {
		return "", errors.New("stop daemon before replacing configuration")
	}
	files, credential, e := generatedConfig(req)
	if e != nil {
		return "", e
	}
	if e := os.MkdirAll(engineBase, 0700); e != nil {
		return "", e
	}
	gate := filepath.Join(engineBase, engine+".gate")
	originals := make(map[string][]byte)
	for _, path := range engineFiles(engine) {
		st, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !st.Mode().IsRegular() || st.Size() > 128<<10 {
			return "", errors.New("refusing non-regular or oversized config")
		}
		if !replace {
			return "", errors.New("existing upstream config: explicit replacement confirmation required")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		originals[path] = b
	}
	var priorGate []byte
	if st, err := os.Lstat(gate); err == nil {
		if !replace || !st.Mode().IsRegular() {
			return "", errors.New("managed gate exists: explicit replacement required")
		}
		priorGate, err = os.ReadFile(gate)
		if err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	for _, path := range engineFiles(engine) {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return "", e
		}
	}
	// Backups are stored outside upstream paths, never overwrite old backups.
	backupDir := filepath.Join(engineBase, "backups", engine)
	if len(originals) > 0 {
		if e := os.MkdirAll(backupDir, 0700); e != nil {
			return "", e
		}
		for _, path := range engineFiles(engine) {
			if old, ok := originals[path]; ok {
				backup := filepath.Join(backupDir, filepath.Base(path)+".before-routerforge")
				if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
					if e := safety.WriteFileAtomic(backup, old, 0600); e != nil {
						return "", e
					}
				} else if err != nil {
					return "", err
				}
			}
		}
	}
	written := []string{}
	restore := func() {
		for _, path := range written {
			if old, ok := originals[path]; ok {
				_ = safety.WriteFileAtomic(path, old, 0600)
			} else {
				_ = os.Remove(path)
			}
		}
	}
	for _, path := range engineFiles(engine) {
		if e := safety.WriteFileAtomic(path, files[path], 0600); e != nil {
			restore()
			return "", e
		}
		written = append(written, path)
	}
	if e := safety.WriteFileAtomic(gate, engineGateConfig(req), 0600); e != nil {
		restore()
		if len(priorGate) > 0 {
			_ = safety.WriteFileAtomic(gate, priorGate, 0600)
		}
		return "", e
	}
	return credential, nil
}

func otherEngine(e string) string {
	if e == "knockd" {
		return "fwknopd"
	}
	return "knockd"
}
func invokeGate(ctx context.Context, action, engine string) error {
	out, e := safety.RunCommand(ctx, 4096, "/bin/sh", engineScript, action, engine)
	if e != nil {
		return fmt.Errorf("gate %s: %s: %w", action, strings.TrimSpace(string(out)), e)
	}
	return nil
}
func enableEngine(engine string) error {
	if engineActive(engine) || engineActive(otherEngine(engine)) || odinEnabled() {
		return errors.New("another protection is active")
	}
	if !fileExists(filepath.Join(engineBase, engine+".gate")) {
		return errors.New("prepare configuration first")
	}
	if serviceSnapshot(engine, "/opt/etc/init.d").Running {
		return errors.New("stop existing daemon before enabling managed protection")
	}
	if !fileExists(engineScript) {
		return errors.New("gate helper missing")
	}
	if _, e := os.Lstat(engineNDMHook); e == nil {
		return errors.New("NDM hook already exists")
	}
	txBytes := make([]byte, 16)
	if _, e := rand.Read(txBytes); e != nil {
		return e
	}
	token := hex.EncodeToString(txBytes)
	tx := filepath.Join(engineBase, "transactions", token)
	if e := os.MkdirAll(filepath.Dir(tx), 0700); e != nil {
		return e
	}
	if e := os.Mkdir(tx, 0700); e != nil {
		return e
	}
	// Watchdog arms before daemon and firewall mutation. Unconfirmed setup is disabled automatically.
	svc, svcErr := findEngineInitScript(engine, "/opt/etc/init.d")
	if svcErr != nil {
		return svcErr
	}
	rollback := "#!/bin/sh\n[ \"$(cat " + engineBase + "/" + engine + ".enabled 2>/dev/null)\" = \"" + token + "\" ] || exit 0\n" + svc + " stop >/dev/null 2>&1 || :\n/bin/sh " + engineScript + " disable " + engine + " || exit 1\nrm -f " + engineBase + "/" + engine + ".enabled \"" + engineNDMHook + "\"\n"

	if e := safety.WriteFileAtomic(filepath.Join(tx, "rollback.sh"), []byte(rollback), 0700); e != nil {
		return e
	}
	cmd, e := safety.CommandContext(context.Background(), odinWatchdog, "-transaction-dir", tx, "-deadline-unix", strconv.FormatInt(time.Now().Add(90*time.Second).Unix(), 10))
	if e != nil {
		return e
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if e = cmd.Start(); e != nil {
		return e
	}
	if e = cmd.Process.Release(); e != nil {
		return e
	}
	// Persist the transaction token before the first firewall mutation:
	// an interrupted process will still be recovered by the external watchdog.
	statePath := filepath.Join(engineBase, engine+".enabled")
	if e := safety.WriteFileAtomic(statePath, []byte(token+"\n"), 0600); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// The scoped DROP gate must exist BEFORE starting either daemon.
	// This is essential for fwknopd: it inserts its dynamic ACCEPT jump
	// inside RFPA_F_GATE, never at the top of the global FORWARD chain.
	if e := invokeGate(ctx, "apply", engine); e != nil {
		cleanup := invokeGate(ctx, "disable", engine)
		if cleanup != nil {
			return fmt.Errorf("%w; cleanup failed: %v", e, cleanup)
		}
		_ = os.Remove(statePath)
		return e
	}
	if e := runExistingEngineService(ctx, engine, "start", "/opt/etc/init.d", invokeEntwareService); e != nil {
		_ = runExistingEngineService(ctx, engine, "stop", "/opt/etc/init.d", invokeEntwareService)
		cleanup := invokeGate(ctx, "disable", engine)
		if cleanup != nil {
			return fmt.Errorf("%w; cleanup failed: %v", e, cleanup)
		}
		_ = os.Remove(statePath)
		return e
	}
	if e := safety.WriteFileAtomic(engineNDMHook, []byte(engineNDMContent), 0755); e != nil {
		_ = runExistingEngineService(ctx, engine, "stop", "/opt/etc/init.d", invokeEntwareService)
		cleanup := invokeGate(ctx, "disable", engine)
		if cleanup != nil {
			return fmt.Errorf("%w; cleanup failed: %v", e, cleanup)
		}
		_ = os.Remove(statePath)
		return e
	}
	return nil
}
func validEngineTransaction(token string) bool {
	if len(token) != 32 {
		return false
	}
	for _, c := range token {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
func confirmEngine(engine string) error {
	b, e := os.ReadFile(filepath.Join(engineBase, engine+".enabled"))
	if e != nil {
		return e
	}
	token := strings.TrimSpace(string(b))
	if !validEngineTransaction(token) {
		return errors.New("invalid transaction")
	}
	return safety.WriteFileAtomic(filepath.Join(engineBase, "transactions", token, "confirmed"), []byte("confirmed\n"), 0600)
}
func disableEngine(engine string) error {
	if !engineActive(engine) {
		return errors.New("protection not enabled")
	}
	var token string
	if b, e := os.ReadFile(filepath.Join(engineBase, engine+".enabled")); e == nil {
		token = strings.TrimSpace(string(b))
	}
	if !validEngineTransaction(token) {
		return errors.New("invalid transaction")
	}
	if b, e := os.ReadFile(engineNDMHook); e == nil {
		if string(b) != engineNDMContent {
			return errors.New("NDM hook changed externally")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if e := invokeGate(ctx, "disable", engine); e != nil {
		return e
	}
	if e := os.Remove(engineNDMHook); e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	if e := os.Remove(filepath.Join(engineBase, engine+".enabled")); e != nil {
		return e
	}
	_ = safety.WriteFileAtomic(filepath.Join(engineBase, "transactions", token, "confirmed"), []byte("disabled\n"), 0600)
	return nil // Engine stop is separate. Stop policy remains preserve.
}
