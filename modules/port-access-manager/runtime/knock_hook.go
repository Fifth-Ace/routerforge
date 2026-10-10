package main

import (
	"errors"
	"regexp"
	"strconv"
)

var wanInterfacePattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.:-]{0,14}$`)

// buildKnockHook is a preview only. It does not run iptables or touch NDM.
// The caller must verify that the port is the post-DNAT destination port.
func buildKnockHook(wan string, targetPort int) ([][]string, [][]string, error) {
	if !wanInterfacePattern.MatchString(wan) || targetPort < 1 || targetPort > 65535 {
		return nil, nil, errors.New("invalid WAN interface or forwarded target port")
	}
	args := []string{"-i", wan, "-p", "tcp", "--dport", strconv.Itoa(targetPort), "-j", "RF_PORT_KNOCK"}
	apply := [][]string{{"iptables", "-t", "filter", "-I", "FORWARD", "1"}}
	apply[0] = append(apply[0], args...)
	rollback := [][]string{{"iptables", "-t", "filter", "-D", "FORWARD"}}
	rollback[0] = append(rollback[0], args...)
	return apply, rollback, nil
}
