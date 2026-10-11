package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Fifth-Ace/routerforge/internal/safety"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const odinDir = "/opt/etc/routerforge/port-access-manager"
const odinConfig = odinDir + "/odin.conf"
const odinState = odinDir + "/odin.enabled"
const odinHook = "/opt/etc/ndm/netfilter.d/100routerforge-odin.sh"
const odinTransaction = odinDir + "/transactions"
const odinWatchdog = "/opt/bin/routerforge-port-access-watchdog"
const odinEngine = "/opt/share/routerforge/modules/port-access-manager/odin-engine.sh"
const odinHookBody = "#!/bin/sh\n# RouterForge odin integration (module-owned)\n[ \"${type:-iptables}\" = ip6tables ] && exit 0\n[ \"${table:-filter}\" = filter ] || exit 0\n[ -f /opt/etc/routerforge/port-access-manager/odin.enabled ] || exit 0\n/bin/sh /opt/share/routerforge/modules/port-access-manager/odin-engine.sh apply || :\nexit 0\n"

var odinMu sync.Mutex
var odinWAN = regexp.MustCompile(`^[a-zA-Z0-9_.:-]{1,24}$`)

type odinSettings struct {
	WAN    string `json:"wan"`
	IP     string `json:"ip"`
	Port   int    `json:"port"`
	Knock  [3]int `json:"knock"`
	Window int    `json:"window"`
	TTL    int    `json:"ttl"`
}

func (s odinSettings) valid() error {
	if !odinWAN.MatchString(s.WAN) {
		return errors.New("invalid WAN interface")
	}
	ip := net.ParseIP(s.IP)
	if ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback() {
		return errors.New("invalid destination IPv4")
	}
	if s.Window < 5 || s.Window > 300 || s.TTL < 30 || s.TTL > 86400 {
		return errors.New("invalid timers")
	}
	seen := map[int]bool{}
	for _, p := range []int{s.Port, s.Knock[0], s.Knock[1], s.Knock[2]} {
		if p < 1 || p > 65535 || seen[p] {
			return errors.New("ports must be unique and within 1..65535")
		}
		seen[p] = true
	}
	return nil
}
func (s odinSettings) data() []byte {
	return []byte(fmt.Sprintf("WAN=%s\nTIP=%s\nTPORT=%d\nK1=%d\nK2=%d\nK3=%d\nWINDOW=%d\nTTL=%d\n", s.WAN, s.IP, s.Port, s.Knock[0], s.Knock[1], s.Knock[2], s.Window, s.TTL))
}
func odinRead() (odinSettings, error) {
	b, e := os.ReadFile(odinConfig)
	if e != nil {
		return odinSettings{}, e
	}
	v := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return odinSettings{}, errors.New("invalid configuration")
		}
		v[parts[0]] = parts[1]
	}
	num := func(k string) int { n, _ := strconv.Atoi(v[k]); return n }
	s := odinSettings{WAN: v["WAN"], IP: v["TIP"], Port: num("TPORT"), Knock: [3]int{num("K1"), num("K2"), num("K3")}, Window: num("WINDOW"), TTL: num("TTL")}
	return s, s.valid()
}
func odinTransactionPath() (string, error) {
	b, err := os.ReadFile(odinState)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(b))
	if len(id) != 32 {
		return "", errors.New("invalid transaction token")
	}
	for _, c := range id {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", errors.New("invalid transaction token")
		}
	}
	return filepath.Join(odinTransaction, id), nil
}
func odinConfirmed() bool {
	path, err := odinTransactionPath()
	if err != nil {
		return false
	}
	info, err := os.Lstat(filepath.Join(path, "confirmed"))
	return err == nil && info.Mode().IsRegular()
}
func odinEnabled() bool { _, err := os.Stat(odinState); return err == nil }
func odinExec(ctx context.Context, action string) error {
	out, err := safety.RunCommand(ctx, 4096, "/bin/sh", odinEngine, action)
	if err != nil {
		return fmt.Errorf("odin %s failed: %s: %w", action, strings.TrimSpace(string(out)), err)
	}
	return nil
}
func odinWriteSettings(s odinSettings) error {
	if err := s.valid(); err != nil {
		return err
	}
	if err := os.MkdirAll(odinDir, 0700); err != nil {
		return err
	}
	return safety.WriteFileAtomic(odinConfig, s.data(), 0600)
}
func odinRemoveHook() error {
	b, err := os.ReadFile(odinHook)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if string(b) != odinHookBody {
		return errors.New("NDM hook changed externally; refusing to remove")
	}
	return os.Remove(odinHook)
}
func odinActivate(ctx context.Context) error {
	if odinEnabled() {
		return errors.New("already enabled")
	}
	if _, err := odinRead(); err != nil {
		return err
	}
	if _, err := os.Lstat(odinHook); err == nil {
		return errors.New("NDM hook already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(odinTransaction, 0700); err != nil {
		return err
	}
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return err
	}
	token := hex.EncodeToString(tokenBytes)
	txDir := filepath.Join(odinTransaction, token)
	if err := os.Mkdir(txDir, 0700); err != nil {
		return err
	}
	rollbackBody := "#!/bin/sh\n[ \"$(cat /opt/etc/routerforge/port-access-manager/odin.enabled 2>/dev/null)\" = \"" + token + "\" ] || exit 0\nrm -f /opt/etc/ndm/netfilter.d/100routerforge-odin.sh\nrm -f /opt/etc/routerforge/port-access-manager/odin.enabled\n/bin/sh /opt/share/routerforge/modules/port-access-manager/odin-engine.sh disable\n"
	if err := safety.WriteFileAtomic(filepath.Join(txDir, "rollback.sh"), []byte(rollbackBody), 0700); err != nil {
		return err
	}
	// An independently running watchdog must be armed before any firewall mutation.
	cmd, err := safety.CommandContext(context.Background(), odinWatchdog, "-transaction-dir", txDir, "-deadline-unix", strconv.FormatInt(time.Now().Add(90*time.Second).Unix(), 10))
	if err != nil {
		return err
	}
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	if err := odinExec(ctx, "enable"); err != nil {
		return err
	}
	rollback := func() {
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = odinExec(c, "disable")
	}
	if err := safety.WriteFileAtomic(odinState, []byte(token+"\n"), 0600); err != nil {
		rollback()
		return err
	}
	if err := safety.WriteFileAtomic(odinHook, []byte(odinHookBody), 0755); err != nil {
		_ = os.Remove(odinState)
		rollback()
		return err
	}
	return nil
}
func odinDeactivate(ctx context.Context) error {
	if txDir, err := odinTransactionPath(); err == nil {
		_ = safety.WriteFileAtomic(filepath.Join(txDir, "confirmed"), []byte("disabled\n"), 0600)
	}
	if err := odinRemoveHook(); err != nil {
		return err
	}
	if err := os.Remove(odinState); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return odinExec(ctx, "disable")
}
func odinHandler(w http.ResponseWriter, r *http.Request) {
	odinMu.Lock()
	defer odinMu.Unlock()
	switch r.Method {
	case http.MethodGet:
		settings, err := odinRead()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			jsonReply(w, 500, map[string]string{"error": err.Error()})
			return
		}
		jsonReply(w, 200, map[string]any{"configured": err == nil, "enabled": odinEnabled(), "confirmed": odinConfirmed(), "settings": settings, "attribution": "odin · Keenetic Community", "source": "https://forum.keenetic.ru/topic/22137-port-knoking-для-пробрасываемого-порта/"})
	case http.MethodPost:
		if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
			jsonReply(w, 403, map[string]string{"error": "Core authorization required"})
			return
		}
		var req struct {
			Action   string       `json:"action"`
			Settings odinSettings `json:"settings"`
			Confirm  string       `json:"confirm"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 4097))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			jsonReply(w, 400, map[string]string{"error": "invalid request"})
			return
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			jsonReply(w, 400, map[string]string{"error": "trailing content"})
			return
		}
		if req.Confirm != "APPLY" {
			jsonReply(w, 400, map[string]string{"error": "confirmation required"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		var err error
		switch req.Action {
		case "save":
			if odinEnabled() {
				err = errors.New("disable engine before editing")
			} else {
				err = odinWriteSettings(req.Settings)
			}
		case "enable":
			err = odinActivate(ctx)
		case "disable":
			err = odinDeactivate(ctx)
		case "confirm":
			if !odinEnabled() {
				err = errors.New("engine is not enabled")
			} else {
				txDir, pathErr := odinTransactionPath()
				if pathErr != nil {
					err = pathErr
				} else {
					err = safety.WriteFileAtomic(filepath.Join(txDir, "confirmed"), []byte("confirmed\n"), 0600)
				}
			}
		default:
			jsonReply(w, 400, map[string]string{"error": "unknown action"})
			return
		}
		if err != nil {
			jsonReply(w, 409, map[string]string{"error": err.Error()})
			return
		}
		jsonReply(w, 200, map[string]any{"ok": true, "enabled": odinEnabled()})
	default:
		w.Header().Set("Allow", "GET, POST")
		jsonReply(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// Keep the hook fixed to a single file; do not scan or manage other NDM hooks.
