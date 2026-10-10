package main

import (
	"errors"
	"strconv"
	"strings"
)

// inspectDNATNDM is a one-hop, read-only analysis of an iptables -t nat -S
// snapshot. Only an unconditional PREROUTING jump to an _NDM_* chain and a
// precisely scoped DNAT rule are recognized. This is evidence, not permission.
func inspectDNATNDM(snapshot, wan string, publicPort, targetPort int) (string, error) {
	if _, _, err := buildKnockHook(wan, targetPort); err != nil {
		return "", err
	}
	if publicPort < 1 || publicPort > 65535 {
		return "", errors.New("invalid external port")
	}
	rules := make([][]string, 0)
	for _, line := range strings.Split(snapshot, "\n") {
		f := strings.Fields(line)
		if len(f) >= 4 && f[0] == "-A" {
			rules = append(rules, f)
		}
	}
	parent := map[string]bool{}
	for _, f := range rules {
		if f[1] != "PREROUTING" {
			continue
		}
		// Only unqualified jumps; selectors on PREROUTING can invalidate the path.
		if len(f) == 4 && f[2] == "-j" && strings.HasPrefix(f[3], "_NDM_") {
			parent[f[3]] = true
		}
	}
	wantedPort := strconv.Itoa(publicPort)
	wantedTarget := strconv.Itoa(targetPort)
	var found string
	for _, f := range rules {
		if !parent[f[1]] {
			continue
		}
		fields := map[string]string{}
		for i := 2; i+1 < len(f); i++ {
			switch f[i] {
			case "-i", "-p", "--dport", "-j", "--to-destination":
				if fields[f[i]] != "" {
					return "", errors.New("ambiguous repeated rule option")
				}
				fields[f[i]] = f[i+1]
				i++
			}
		}
		if fields["-i"] != wan || fields["-p"] != "tcp" || fields["--dport"] != wantedPort || fields["-j"] != "DNAT" {
			continue
		}
		dest := strings.Split(fields["--to-destination"], ":")
		if len(dest) != 2 || dest[0] == "" || dest[1] != wantedTarget {
			continue
		}
		if found != "" {
			return "", errors.New("multiple matching DNAT paths")
		}
		found = dest[0]
	}
	if found == "" {
		return "", errors.New("one-hop NDM DNAT path not proven")
	}
	return found, nil
}
