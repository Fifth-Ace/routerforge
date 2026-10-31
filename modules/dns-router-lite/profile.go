package lite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"

	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

const profileName = "RF_DNS_LITE"

type profileState struct {
	State       string `json:"state"`
	MAC         string `json:"mac"`
	Listen      string `json:"listen"`
	Previous    string `json:"previous"`
	Engine      string `json:"engine"`
	ForeignHash string `json:"foreign_hash"`
	Persist     bool   `json:"persist"`
}
type ProfileManager struct {
	mu        sync.Mutex
	Path      string
	Executor  dr.NDMCExecutorM
	lastError string
}

func (p *ProfileManager) command(ctx context.Context, c string) (string, error) {
	out, e := p.Executor.Command(ctx, c)
	if e != nil {
		return "", e
	}
	lower := strings.ToLower(out)
	if strings.Contains(lower, "failed") || strings.Contains(lower, "error:") || strings.Contains(lower, "not found") || strings.Contains(lower, "invalid command") {
		return "", errors.New("Keenetic rejected DNS profile command")
	}
	return out, nil
}
func (p *ProfileManager) load() (*profileState, error) {
	fi, e := os.Lstat(p.Path)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !fi.Mode().IsRegular() || fi.Size() > 8192 {
		return nil, errors.New("invalid DNS connection state")
	}
	b, e := os.ReadFile(p.Path)
	if e != nil {
		return nil, e
	}
	var s profileState
	e = json.Unmarshal(b, &s)
	if e != nil {
		return nil, e
	}
	mac, e := canonicalMAC(s.MAC)
	if e != nil || mac != s.MAC || s.ForeignHash == "" {
		return nil, errors.New("invalid DNS connection ownership")
	}
	return &s, nil
}
func canonicalMAC(s string) (string, error) {
	a, e := net.ParseMAC(s)
	if e != nil || len(a) != 6 || a[0]&1 != 0 {
		return "", errors.New("enter a unicast client MAC address")
	}
	return strings.ToLower(a.String()), nil
}

// Only DNS commands are retained from running-config. Passwords and unrelated
// configuration never enter a state file, API response or diagnostic log.
func dnsLines(raw string) ([]string, error) {
	if len(raw) > 65536 || (!strings.Contains(raw, "hostname ") && !strings.Contains(raw, "dns-proxy\n") && !strings.Contains(raw, "dns-proxy\r\n")) {
		return nil, errors.New("running-config format is not recognized; no DNS changes made")
	}
	lines := []string{}
	inDNS := false
	profile := ""
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "dns-proxy ") {
			lines = append(lines, strings.TrimPrefix(t, "dns-proxy "))
			continue
		}
		if t == "dns-proxy" {
			inDNS = true
			profile = ""
			continue
		}
		if t == "!" {
			profile = ""
			continue
		}
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if inDNS && len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			inDNS = false
		}
		if !inDNS {
			continue
		}
		if strings.HasPrefix(t, "filter profile ") {
			f := strings.Fields(t)
			if len(f) == 3 {
				profile = t
			} else {
				profile = ""
			}
		}
		if profile != "" && (strings.HasPrefix(t, "dns53 upstream ") || strings.HasPrefix(t, "https upstream ") || strings.HasPrefix(t, "tls upstream ") || strings.HasPrefix(t, "intercept ")) {
			t = profile + " " + t
		}
		fields := strings.Fields(t)
		if len(fields) >= 5 && fields[0] == "filter" && fields[1] == "assign" && fields[2] == "host" {
			fields[4] = strings.ToLower(fields[4])
		}
		if len(fields) == 7 && fields[0] == "filter" && fields[1] == "profile" && fields[3] == "dns53" && fields[4] == "upstream" {
			fields = append(fields[:5], fields[5]+":"+fields[6])
		}
		lines = append(lines, strings.Join(fields, " "))
	}
	return lines, nil
}
func assignment(line, mac string) bool {
	f := strings.Fields(strings.ToLower(line))
	return len(f) >= 5 && f[0] == "filter" && f[1] == "assign" && f[2] == "host" && (f[3] == "profile" || f[3] == "preset") && f[4] == mac
}
func ownedLine(line string) bool {
	return line == "filter profile "+profileName || strings.HasPrefix(line, "filter profile "+profileName+" ")
}
func inventory(lines []string, mac string) (foreign, previous, engine string, owned []string, err error) {
	other := []string{}
	for _, l := range lines {
		if ownedLine(l) {
			owned = append(owned, l)
			continue
		}
		if assignment(l, mac) {
			if previous != "" {
				err = errors.New("multiple client DNS assignments")
				return
			}
			previous = l
			continue
		}
		if strings.HasPrefix(l, "filter engine ") {
			if engine != "" {
				err = errors.New("multiple DNS engines")
				return
			}
			engine = l
			continue
		}
		other = append(other, l)
	}
	sort.Strings(other)
	h := sha256.Sum256([]byte(strings.Join(other, "\n")))
	foreign = hex.EncodeToString(h[:])
	return
}
func (p *ProfileManager) read(ctx context.Context) ([]string, error) {
	raw, e := p.command(ctx, "show running-config")
	if e != nil {
		return nil, e
	}
	return dnsLines(raw)
}
func (p *ProfileManager) Status() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastError != "" {
		return map[string]any{"state": "recovery_required", "error": p.lastError}
	}
	s, e := p.load()
	if e != nil {
		return map[string]any{"state": "recovery_required", "error": e.Error()}
	}
	if s == nil {
		return map[string]any{"state": "disconnected"}
	}
	return map[string]any{"state": s.State, "mac": s.MAC, "listen": s.Listen, "persistent": s.Persist, "hardware_route_verified": false}
}

// Reconcile only reads native state. A non-persistent profile lost at reboot
// clears its local ownership marker once the exact baseline is observed.
func (p *ProfileManager) Reconcile(ctx context.Context) (err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() {
		if err != nil {
			p.lastError = err.Error()
		}
	}()
	s, e := p.load()
	if e != nil || s == nil {
		return e
	}
	lines, e := p.read(ctx)
	if e != nil {
		return e
	}
	f, a, engine, owned, e := inventory(lines, s.MAC)
	if e != nil {
		return e
	}
	if f == s.ForeignHash && a == s.Previous && engine == s.Engine && len(owned) == 0 {
		return os.Remove(p.Path)
	}
	if f != s.ForeignHash || engine != s.Engine || a != "filter assign host profile "+s.MAC+" "+profileName || !exactOwned(owned, s.Listen) {
		return errors.New("client DNS ownership changed; inspect connection before restoring it")
	}
	return nil
}
func (p *ProfileManager) Connected() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, e := p.load()
	return e != nil || s != nil
}
func (p *ProfileManager) Connect(ctx context.Context, mac, listen string, persist bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	mac, e := canonicalMAC(mac)
	if e != nil {
		return e
	}
	if s, e := p.load(); e != nil {
		return e
	} else if s != nil {
		return errors.New("disconnect existing owned client first")
	}
	lines, e := p.read(ctx)
	if e != nil {
		return e
	}
	f, prev, engine, owned, e := inventory(lines, mac)
	if e != nil {
		return e
	}
	if len(owned) > 0 {
		return errors.New("reserved RF_DNS_LITE profile already exists; refusing to overwrite")
	}
	if prev != "" {
		fields := strings.Fields(prev)
		if len(fields) != 6 || strings.ContainsAny(fields[5], "\"'\\;\r\n") {
			return errors.New("previous DNS profile identifier cannot be restored unambiguously")
		}
	}
	if engine != "" && engine != "filter engine public" {
		return errors.New("another DNS filtering engine is active; keep its configuration and choose public/custom profiles explicitly in Keenetic first")
	}
	s := &profileState{State: "pending", MAC: mac, Listen: listen, Previous: prev, Engine: engine, ForeignHash: f, Persist: persist}
	b, _ := json.Marshal(s)
	if e = atomicFile(p.Path, b); e != nil {
		return e
	}
	commands := []string{"dns-proxy filter profile " + profileName, "dns-proxy filter profile " + profileName + " dns53 upstream " + listen}
	if prev != "" {
		kind := strings.Fields(prev)[3]
		commands = append(commands, "no dns-proxy filter assign host "+kind+" "+mac)
	}
	commands = append(commands, "dns-proxy filter assign host profile "+mac+" "+profileName)
	for _, c := range commands {
		if _, e = p.command(ctx, c); e != nil {
			break
		}
	}
	if e == nil {
		var ls []string
		ls, e = p.read(ctx)
		if e == nil {
			ff, a, en, oo, er := inventory(ls, mac)
			e = er
			if e == nil && (ff != f || a != "filter assign host profile "+mac+" "+profileName || en != engine || !exactOwned(oo, listen)) {
				e = errors.New("Keenetic did not confirm the exact client DNS profile")
			}
		}
	}
	if e == nil && persist {
		_, e = p.command(ctx, "system configuration save")
	}
	if e != nil {
		original := e
		if re := p.disconnectLocked(context.Background()); re != nil {
			return fmt.Errorf("connect failed: %v; recovery required: %v", original, re)
		}
		return fmt.Errorf("connect failed; baseline restored: %w", original)
	}
	s.State = "connected"
	b, _ = json.Marshal(s)
	if e = atomicFile(p.Path, b); e != nil {
		return e
	}
	p.lastError = ""
	return nil
}
func exactOwned(lines []string, listen string) bool {
	if len(lines) != 2 {
		return false
	}
	a := append([]string(nil), lines...)
	sort.Strings(a)
	return a[0] == "filter profile "+profileName && a[1] == "filter profile "+profileName+" dns53 upstream "+listen
}
func (p *ProfileManager) Disconnect(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.disconnectLocked(ctx)
}
func (p *ProfileManager) disconnectLocked(ctx context.Context) error {
	s, e := p.load()
	if e != nil || s == nil {
		return e
	}
	lines, e := p.read(ctx)
	if e != nil {
		return e
	}
	f, a, engine, owned, e := inventory(lines, s.MAC)
	if e != nil {
		return e
	}
	if f != s.ForeignHash {
		return errors.New("other DNS settings changed; inspect ownership before disconnecting")
	}
	if a != "" && a != s.Previous && a != "filter assign host profile "+s.MAC+" "+profileName {
		return errors.New("client DNS assignment changed outside RouterForge")
	}
	if engine != s.Engine {
		return errors.New("DNS engine changed outside RouterForge")
	}
	for _, l := range owned {
		if l != "filter profile "+profileName && l != "filter profile "+profileName+" dns53 upstream "+s.Listen {
			return errors.New("owned DNS profile was modified outside RouterForge")
		}
	}
	if a == "filter assign host profile "+s.MAC+" "+profileName {
		if _, e = p.command(ctx, "no dns-proxy filter assign host profile "+s.MAC); e != nil {
			return e
		}
	}
	if s.Previous != "" && a != s.Previous {
		if _, e = p.command(ctx, "dns-proxy "+s.Previous); e != nil {
			return e
		}
	}
	if len(owned) > 0 {
		if _, e = p.command(ctx, "no dns-proxy filter profile "+profileName); e != nil {
			return e
		}
	}
	lines, e = p.read(ctx)
	if e != nil {
		return e
	}
	ff, aa, ee, oo, e := inventory(lines, s.MAC)
	if e != nil {
		return e
	}
	if ff != s.ForeignHash || aa != s.Previous || ee != s.Engine || len(oo) != 0 {
		return errors.New("DNS profile baseline restoration not confirmed")
	}
	if s.Persist {
		if _, e = p.command(ctx, "system configuration save"); e != nil {
			return e
		}
	}
	e = os.Remove(p.Path)
	if e == nil {
		p.lastError = ""
	}
	return e
}
