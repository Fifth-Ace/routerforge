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
	WAN     string   `json:"wan"`
	IP      string   `json:"ip"`
	Port    int      `json:"port"`
	Ports   []int    `json:"ports,omitempty"`
	Trusted []string `json:"trusted,omitempty"`
	Knock   [3]int   `json:"knock"`
	Window  int      `json:"window"`
	TTL     int      `json:"ttl"`
}

func (s odinSettings) protectedPorts() ([]int, error) {
	ports := s.Ports
	if len(ports) == 0 {
		ports = []int{s.Port}
	}
	if len(ports) > 15 || len(ports) == 0 {
		return nil, errors.New("protected port count must be 1..15")
	}
	if s.Port != 0 && s.Port != ports[0] {
		return nil, errors.New("legacy port must equal first protected port")
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			return nil, errors.New("protected ports must be unique 1..65535")
		}
		seen[p] = true
	}
	return ports, nil
}
func (s odinSettings) trustedAddresses() ([]string, error) {
	if len(s.Trusted) > 32 {
		return nil, errors.New("too many trusted addresses")
	}
	out := make([]string, 0, len(s.Trusted))
	seen := map[string]bool{}
	for _, raw := range s.Trusted {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return nil, errors.New("empty trusted address")
		}
		var ip net.IP
		var network *net.IPNet
		var err error
		if strings.Contains(raw, "/") {
			ip, network, err = net.ParseCIDR(raw)
			if err != nil {
				return nil, errors.New("invalid trusted CIDR")
			}
			if ip.To4() == nil || !ip.Equal(network.IP) {
				return nil, errors.New("trusted CIDR must be canonical IPv4 network")
			}
		} else {
			ip = net.ParseIP(raw)
			if ip == nil || ip.To4() == nil {
				return nil, errors.New("invalid trusted IPv4")
			}
			raw = ip.To4().String() + "/32"
		}
		if network != nil {
			raw = network.String()
		}
		if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLoopback() || raw == "0.0.0.0/0" || seen[raw] {
			return nil, errors.New("unsafe or duplicate trusted address")
		}
		seen[raw] = true
		out = append(out, raw)
	}
	return out, nil
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
	ports, err := s.protectedPorts()
	if err != nil {
		return err
	}
	if _, err = s.trustedAddresses(); err != nil {
		return err
	}
	seen := map[int]bool{}
	for _, p := range s.Knock {
		if p < 1 || p > 65535 || seen[p] {
			return errors.New("invalid knock ports")
		}
		seen[p] = true
	}
	for _, p := range ports {
		if seen[p] {
			return errors.New("protected and knock ports overlap")
		}
	}
	return nil
}

func (s odinSettings) data() []byte {
	ports, _ := s.protectedPorts()
	trusted, _ := s.trustedAddresses()
	pp := make([]string, len(ports))
	for i, p := range ports {
		pp[i] = strconv.Itoa(p)
	}
	return []byte(fmt.Sprintf("WAN=%s\nTIP=%s\nTPORT=%d\nTPORTS=%s\nTRUSTED=%s\nK1=%d\nK2=%d\nK3=%d\nWINDOW=%d\nTTL=%d\n", s.WAN, s.IP, ports[0], strings.Join(pp, ","), strings.Join(trusted, ","), s.Knock[0], s.Knock[1], s.Knock[2], s.Window, s.TTL))
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
	if raw := v["TPORTS"]; raw != "" {
		for _, part := range strings.Split(raw, ",") {
			p, err := strconv.Atoi(part)
			if err != nil {
				return odinSettings{}, errors.New("invalid protected port list")
			}
			s.Ports = append(s.Ports, p)
		}
	}
	if raw := v["TRUSTED"]; raw != "" {
		s.Trusted = strings.Split(raw, ",")
	}
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
func odinWatchdogDeadline() int64 {
	if !odinEnabled() || odinConfirmed() {
		return 0
	}
	tx, err := odinTransactionPath()
	if err != nil {
		return 0
	}
	raw, err := os.ReadFile(filepath.Join(tx, "deadline-unix"))
	if err != nil {
		return 0
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}
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

// odin requires xt_comment for the original NDM chain ownership markers.
// Check the kernel registry before arming the watchdog or touching firewall.
func odinRequireComment(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot check kernel iptables matches: %w", err)
	}
	for _, name := range strings.Fields(string(data)) {
		if name == "comment" {
			return nil
		}
	}
	return errors.New("\u041e\u0442\u0441\u0443\u0442\u0441\u0442\u0432\u0443\u0435\u0442 \u0437\u0430\u0433\u0440\u0443\u0436\u0435\u043d\u043d\u044b\u0439 \u043c\u043e\u0434\u0443\u043b\u044c \u044f\u0434\u0440\u0430 xt_comment (iptables comment). \u0423\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u0435 \u0441\u043e\u0432\u043c\u0435\u0441\u0442\u0438\u043c\u044b\u0439 \u0441 Keenetic \u043c\u043e\u0434\u0443\u043b\u044c xt_comment \u0438 \u0437\u0430\u0433\u0440\u0443\u0437\u0438\u0442\u0435 \u0435\u0433\u043e. \u0417\u0430\u0442\u0435\u043c \u043f\u043e\u0432\u0442\u043e\u0440\u0438\u0442\u0435 \u0432\u043a\u043b\u044e\u0447\u0435\u043d\u0438\u0435 \u0437\u0430\u0449\u0438\u0442\u044b. RouterForge \u043d\u0435 \u0438\u0437\u043c\u0435\u043d\u044f\u043b firewall.")
}
func odinActivate(ctx context.Context) error {
	if odinEnabled() {
		return errors.New("already enabled")
	}
	if _, err := odinRead(); err != nil {
		return err
	}
	if err := odinRequireComment("/proc/net/ip_tables_matches"); err != nil {
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
	deadline := time.Now().Add(90 * time.Second).Unix()
	if err := safety.WriteFileAtomic(filepath.Join(txDir, "deadline-unix"), []byte(strconv.FormatInt(deadline, 10)+"\n"), 0600); err != nil {
		return err
	}
	cmd, err := safety.CommandContext(context.Background(), odinWatchdog, "-transaction-dir", txDir, "-deadline-unix", strconv.FormatInt(deadline, 10))
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
		// Always try to remove partially installed chains after failed apply.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		cleanupErr := odinExec(cleanupCtx, "disable")
		cleanupCancel()
		if cleanupErr != nil {
			return fmt.Errorf("%w; firewall cleanup failed: %v", err, cleanupErr)
		}
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
		jsonReply(w, 200, map[string]any{"configured": err == nil, "enabled": odinEnabled(), "confirmed": odinConfirmed(), "watchdog_deadline_unix": odinWatchdogDeadline(), "server_unix": time.Now().Unix(), "settings": settings, "attribution": "odin · Keenetic Community", "source": "https://forum.keenetic.ru/topic/22137-port-knoking-для-пробрасываемого-порта/"})
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
