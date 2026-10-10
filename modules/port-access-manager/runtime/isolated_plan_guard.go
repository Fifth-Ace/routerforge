package main

import (
	"errors"
	"reflect"
	"strconv"
)

// validateCanonicalIsolatedPlan refuses hand-written iptables arguments even
// when their chain names are allowlisted. Only the exact output of the
// validated RouterForge generator may reach the isolated executor.
func validateCanonicalIsolatedPlan(commands [][]string) error {
	if len(commands) != 10 {
		return errors.New("noncanonical isolated plan length")
	}
	port := func(i int) (int, error) {
		cmd := commands[i]
		found := false
		value := 0
		for j := 0; j < len(cmd); j++ {
			if cmd[j] != "--dport" {
				continue
			}
			if found || j+1 >= len(cmd) {
				return 0, errors.New("ambiguous target port")
			}
			n, err := strconv.Atoi(cmd[j+1])
			if err != nil {
				return 0, err
			}
			value = n
			found = true
		}
		if !found {
			return 0, errors.New("missing target port")
		}
		return value, nil
	}
	seconds := func(i int) (int, error) {
		cmd := commands[i]
		found := false
		value := 0
		for j := 0; j < len(cmd); j++ {
			if cmd[j] != "--seconds" {
				continue
			}
			if found || j+1 >= len(cmd) {
				return 0, errors.New("ambiguous seconds")
			}
			n, err := strconv.Atoi(cmd[j+1])
			if err != nil {
				return 0, err
			}
			value = n
			found = true
		}
		if !found {
			return 0, errors.New("missing seconds")
		}
		return value, nil
	}
	var o KnockOptions
	for step := 0; step < 3; step++ {
		p, err := port(step + 3)
		if err != nil {
			return err
		}
		o.Sequence[step] = p
	}
	var err error
	o.Target, err = port(6)
	if err != nil {
		return err
	}
	o.WindowSeconds, err = seconds(4)
	if err != nil {
		return err
	}
	secondWindow, err := seconds(5)
	if err != nil {
		return err
	}
	if secondWindow != o.WindowSeconds {
		return errors.New("inconsistent knock windows")
	}
	o.AccessSeconds, err = seconds(6)
	if err != nil {
		return err
	}
	canonical, err := buildKnockRules(o)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(commands, canonical.Commands) {
		return errors.New("noncanonical isolated iptables commands")
	}
	return validateIsolatedPlan(commands)
}
