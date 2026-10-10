package main

import (
	"errors"
	"strings"
)

// checkForwardOrder only analyzes an iptables -S filter snapshot.
// It never authorizes installation or changes any rule.
func checkForwardOrder(snapshot, wan string, port int) (int, error) {
	if _, _, err := buildKnockHook(wan, port); err != nil {
		return 0, err
	}
	position := 0
	chainPresent := false
	for _, raw := range strings.Split(snapshot, "\n") {
		fields := strings.Fields(raw)
		if len(fields) >= 3 && fields[0] == "-P" && fields[1] == "FORWARD" {
			chainPresent = true
		}
		if len(fields) < 3 || fields[0] != "-A" || fields[1] != "FORWARD" {
			continue
		}
		position++
		chainPresent = true
		for i := 2; i+1 < len(fields); i++ {
			if fields[i] == "-j" && fields[i+1] == "RF_PORT_KNOCK" {
				return 0, errors.New("RouterForge hook already exists; refusing duplicate")
			}
		}
	}
	// The existing K4D insertion plan uses position 1. If there are
	// no FORWARD rules, this still means the first rule in that chain.
	if !chainPresent {
		return 0, errors.New("FORWARD chain absent from snapshot")
	}
	return 1, nil
}
