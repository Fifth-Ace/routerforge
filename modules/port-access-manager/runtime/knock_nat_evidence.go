package main

import (
	"errors"
	"strconv"
	"strings"
)

// inspectDNATDirect reads only an iptables -t nat -S snapshot.
// A custom NDM chain or unsupported rule remains unknown, never verified.
func inspectDNATDirect(snapshot, wan string, publicPort, targetPort int) (string, error) {
	if _, _, err := buildKnockHook(wan, targetPort); err != nil {
		return "", err
	}
	if publicPort < 1 || publicPort > 65535 {
		return "", errors.New("invalid external port")
	}
	for _, line := range strings.Split(snapshot, "\n") {
		f := strings.Fields(line)
		if len(f) < 12 || f[0] != "-A" || f[1] != "PREROUTING" {
			continue
		}
		args := map[string]string{}
		for i := 2; i+1 < len(f); i++ {
			switch f[i] {
			case "-i", "-p", "--dport", "-j", "--to-destination":
				args[f[i]] = f[i+1]
				i++
			}
		}
		if args["-i"] != wan || args["-p"] != "tcp" || args["--dport"] != strconv.Itoa(publicPort) || args["-j"] != "DNAT" {
			continue
		}
		parts := strings.Split(args["--to-destination"], ":")
		if len(parts) == 2 && parts[0] != "" && parts[1] == strconv.Itoa(targetPort) {
			return parts[0], nil
		}
	}
	return "", errors.New("direct PREROUTING DNAT not proven; custom NDM chains require separate inspection")
}
