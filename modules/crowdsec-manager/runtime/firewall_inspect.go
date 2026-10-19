package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Inspection only. Never mutate a user-owned firewall or execute a hook.
type firewallInspection struct {
	OK     bool
	Detail string
	Chains []string
	Chain  string
}

var externalCrowdSecHook = regexp.MustCompile(`^(?:[0-9]{3}-)?crowdsec\.sh$`)

func crowdSecHook() (string, bool, bool) {
	const dir = "/opt/etc/ndm/netfilter.d"
	for _, name := range []string{"crowdsec.sh", "001-crowdsec.sh"} {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return path, true, name == "crowdsec.sh"
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, false
	}
	for _, ent := range entries {
		if !externalCrowdSecHook.MatchString(ent.Name()) {
			continue
		}
		path := filepath.Join(dir, ent.Name())
		info, e := os.Lstat(path)
		if e == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return path, true, ent.Name() == "crowdsec.sh"
		}
	}
	return "", false, false
}

func hasJump(rules, from, to string) bool {
	for _, line := range strings.Split(rules, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "-A" || fields[1] != from {
			continue
		}
		for i := 2; i+1 < len(fields); i++ {
			if (fields[i] == "-j" || fields[i] == "-g") && fields[i+1] == to {
				return true
			}
		}
	}
	return false
}

func hasCrowdSecDrop(rules, chain string, sets []string) bool {
	valid := make(map[string]bool, len(sets))
	for _, name := range sets {
		valid[name] = true
	}
	for _, line := range strings.Split(rules, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 || fields[0] != "-A" || fields[1] != chain {
			continue
		}
		set, direction, verdict := "", "", ""
		for i := 2; i+1 < len(fields); i++ {
			switch fields[i] {
			case "--match-set":
				if i+2 < len(fields) {
					set = fields[i+1]
					direction = fields[i+2]
				}
			case "-j":
				verdict = fields[i+1]
			}
		}
		if valid[set] && direction == "src" && verdict == "DROP" {
			return true
		}
	}
	return false
}

func jumpIndex(rules, from, to string) int {
	index := 0
	for _, line := range strings.Split(rules, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "-A" || fields[1] != from {
			continue
		}
		index++
		for i := 2; i+1 < len(fields); i++ {
			if (fields[i] == "-j" || fields[i] == "-g") && fields[i+1] == to {
				return index
			}
		}
	}
	return -1
}

func hasEstablishedAcceptBefore(rules string, position int) bool {
	if position < 1 {
		return false
	}
	index := 0
	for _, line := range strings.Split(rules, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "-A" || fields[1] != "FORWARD" {
			continue
		}
		index++
		if index >= position {
			return false
		}
		state, accept := false, false
		for i := 2; i < len(fields); i++ {
			if (fields[i] == "--state" || fields[i] == "--ctstate") && i+1 < len(fields) {
				state = strings.Contains(fields[i+1], "ESTABLISHED") && strings.Contains(fields[i+1], "RELATED")
			}
			if fields[i] == "-j" && i+1 < len(fields) && fields[i+1] == "ACCEPT" {
				accept = true
			}
		}
		if state && accept {
			return true
		}
	}
	return false
}

func analyzeCrowdSecFirewall(forward, ndmForward, standard, custom string, sets []string) firewallInspection {
	candidates := []struct{ name, rules string }{{"CROWDSEC_CHAIN", standard}, {"_CROWDSEC_CHAIN", custom}}
	for _, c := range candidates {
		if !hasCrowdSecDrop(c.rules, c.name, sets) {
			continue
		}
		direct := hasJump(forward, "FORWARD", c.name)
		nested := hasJump(forward, "FORWARD", "_NDM_FORWARD") && hasJump(ndmForward, "_NDM_FORWARD", c.name)
		if !direct && !nested {
			continue
		}
		path := "FORWARD → " + c.name
		if nested && !direct {
			path = "FORWARD → _NDM_FORWARD → " + c.name
		}
		result := firewallInspection{OK: true, Chain: c.name, Chains: []string{path, c.name + " → ipset DROP"}, Detail: "Подтверждена связка " + path + " → ipset DROP"}
		if direct && c.name == "_CROWDSEC_CHAIN" {
			own := jumpIndex(forward, "FORWARD", c.name)
			ndm := jumpIndex(forward, "FORWARD", "_NDM_FORWARD")
			if ndm > 0 && own >= ndm {
				result.OK = false
				result.Detail = "CrowdSec расположен после _NDM_FORWARD: достижимость правил не подтверждена"
			} else if hasEstablishedAcceptBefore(forward, own) {
				result.Detail += "; правило расположено после RELATED,ESTABLISHED"
			} else {
				result.Detail += "; позиция относительно RELATED,ESTABLISHED не подтверждена"
			}
		}
		if nested && !direct {
			result.Detail += "; фильтрация зависит от достижимости _NDM_FORWARD"
		}
		return result
	}
	return firewallInspection{Detail: "Не подтверждена связка FORWARD → CrowdSec → ipset DROP (включая пользовательские цепочки)"}
}

func inspectCrowdSecFirewall(ctx context.Context, sets []string) firewallInspection {
	read := func(chain string) string {
		result, err := bounded(ctx, "/usr/sbin/iptables", "-S", chain)
		if err != nil {
			result, err = bounded(ctx, "/opt/sbin/iptables", "-S", chain)
		}
		if err != nil {
			return ""
		}
		return result
	}
	return analyzeCrowdSecFirewall(read("FORWARD"), read("_NDM_FORWARD"), read("CROWDSEC_CHAIN"), read("_CROWDSEC_CHAIN"), sets)
}

// An external hook takes precedence for write protection, even if it is not executable.
func externalCrowdSecHookExists() bool {
	entries, err := os.ReadDir("/opt/etc/ndm/netfilter.d")
	if err != nil {
		return false
	}
	for _, ent := range entries {
		if ent.Name() == "crowdsec.sh" || !externalCrowdSecHook.MatchString(ent.Name()) {
			continue
		}
		info, err := os.Lstat(filepath.Join("/opt/etc/ndm/netfilter.d", ent.Name()))
		if err == nil && (info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0) {
			return true
		}
	}
	return false
}
