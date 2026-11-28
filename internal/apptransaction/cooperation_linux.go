//go:build linux

package apptransaction

import (
	"errors"
	"fmt"
)

// CooperativeVersionSet is an exact authoritative inventory of all eleven
// official packages. Empty version means positively proven absent, not unknown.
// Callers must separately reject legacy aliases, unknown cooperating binaries,
// untrusted package metadata and in-flight operations. A version string alone
// never proves that the running process implements the cooperative protocol.
type CooperativeVersionSet map[string]string

func validCooperativeSet(s CooperativeVersionSet) error {
	if len(s) != len(official) || s["routerforge-core"] == "" {
		return errors.New("incomplete cooperative inventory")
	}
	for name, version := range s {
		if !official[name] {
			return fmt.Errorf("unsupported cooperative package: %s", name)
		}
		if version != "" {
			if _, e := devRevision(version); e != nil {
				return fmt.Errorf("invalid cooperative version: %s", name)
			}
		}
	}
	return nil
}
func equalCooperativeSets(a, b CooperativeVersionSet) bool {
	for name, v := range a {
		if b[name] != v {
			return false
		}
	}
	return len(a) == len(b)
}

// CheckCooperativeTransition rejects an unaccepted initial or intermediate
// package vector, including the final state. accepted must be supplied from
// trusted, immutable hardware acceptance records, never a browser or merely a
// fresh release index. No accepted real-device matrix exists in this delivery.
// This policy is not wired into an executable Worker yet.
func CheckCooperativeTransition(plan Plan, installed CooperativeVersionSet, accepted []CooperativeVersionSet) error {
	if _, e := plan.Hash(); e != nil {
		return e
	}
	if e := validCooperativeSet(installed); e != nil {
		return e
	}
	if len(accepted) == 0 || len(accepted) > 256 {
		return errors.New("no bounded accepted compatibility matrix")
	}
	for _, state := range accepted {
		if e := validCooperativeSet(state); e != nil {
			return e
		}
	}
	current := CooperativeVersionSet{}
	for name, v := range installed {
		current[name] = v
	}
	admitted := func() bool {
		for _, state := range accepted {
			if equalCooperativeSets(current, state) {
				return true
			}
		}
		return false
	}
	if !admitted() {
		return errors.New("initial package version vector is not accepted")
	}
	for _, p := range plan.Packages {
		old := current[p.Name]
		if old == "" || old == p.Version {
			return fmt.Errorf("selected package is not a forward installed upgrade: %s", p.Name)
		}
		if e := allowUpgrade(old, p.Version); e != nil {
			return e
		}
		current[p.Name] = p.Version
		if !admitted() {
			return fmt.Errorf("unaccepted intermediate package vector after %s", p.Name)
		}
	}
	return nil
}
