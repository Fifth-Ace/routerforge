package main

import (
	"bufio"
	"errors"
	"net/netip"
	"strings"
)

func v2StrategySelectionArg(arg string) bool {
	switch {
	case strings.HasPrefix(arg, "--hostlist-domains="),
		strings.HasPrefix(arg, "--hostlist="),
		strings.HasPrefix(arg, "--hostlist-auto="),
		strings.HasPrefix(arg, "--hostlist-exclude="),
		strings.HasPrefix(arg, "--ipset="),
		strings.HasPrefix(arg, "--ipset-exclude="):
		return true
	default:
		return false
	}
}

func v2ProductionSourceProfileEligible(profile benchStrategyProfile) bool {
	if profile.Index < 0 || len(profile.Args) == 0 {
		return false
	}
	return validatePreviewArgs(profile.Args) == nil
}

func v2BindCandidateToSourceProfile(source benchStrategyProfile, testedArgs []string) ([]string, error) {
	if !v2ProductionSourceProfileEligible(source) {
		return nil, errors.New("source production profile is not eligible")
	}
	technique := v2PortableCandidateArgs(testedArgs)
	if len(technique) == 0 {
		return nil, errors.New("candidate technique is empty")
	}
	techniqueProfile := analyzeBenchStrategyProfile(-1, technique)
	if !techniqueProfile.CandidateEligible {
		return nil, errors.New("candidate technique is not eligible: " + strings.Join(techniqueProfile.Reasons, "; "))
	}
	bound := make([]string, 0, len(source.Args)+len(technique))
	for _, arg := range source.Args {
		if v2StrategySelectionArg(arg) {
			bound = append(bound, arg)
		}
	}
	bound = append(bound, technique...)
	if err := validatePreviewArgs(bound); err != nil {
		return nil, errors.New("bound candidate is invalid: " + err.Error())
	}
	return bound, nil
}

func v2ListContainsIPv4(text, destinationIPv4 string) bool {
	address, err := netip.ParseAddr(strings.TrimSpace(destinationIPv4))
	if err != nil || !address.Is4() {
		return false
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if cut := strings.IndexAny(line, " \t#;"); cut >= 0 {
			line = strings.TrimSpace(line[:cut])
		}
		line = strings.Trim(line, "\"'")
		if prefix, prefixErr := netip.ParsePrefix(line); prefixErr == nil {
			if prefix.Addr().Is4() && prefix.Contains(address) {
				return true
			}
			continue
		}
		if item, itemErr := netip.ParseAddr(line); itemErr == nil && item.Is4() && item == address {
			return true
		}
	}
	return false
}

func v2ProfileMatchesTargetWithLists(
	target string,
	profile benchStrategyProfile,
	domainMatches map[string]bool,
	ipMatches map[string]bool,
) bool {
	matched := false
	hasPositiveSelector := false
	for _, domain := range profile.HostlistDomains {
		hasPositiveSelector = true
		if v2DomainMatches(target, domain) {
			matched = true
		}
	}
	for _, arg := range profile.FileBoundFilters {
		name, relation := v2ProfileFileRelation(arg)
		if name == "" {
			continue
		}
		switch relation {
		case "exclude":
			if domainMatches[name] {
				return false
			}
		case "ipset-exclude":
			if ipMatches[name] {
				return false
			}
		case "ipset":
			hasPositiveSelector = true
			if ipMatches[name] {
				matched = true
			}
		default:
			hasPositiveSelector = true
			if domainMatches[name] {
				matched = true
			}
		}
	}
	if !hasPositiveSelector {
		return true
	}
	return matched
}

func v2ProductionProfilesMatchingTarget(target, destinationIPv4 string, inventory benchStrategyInventory) []benchStrategyProfile {
	domainMatches := map[string]bool{}
	ipMatches := map[string]bool{}
	for _, item := range readStatus().Lists {
		data, _, err := v2ReadListByName(item.Name)
		if err != nil {
			continue
		}
		text := string(data)
		if v2ListContainsTarget(text, target) {
			domainMatches[item.Name] = true
		}
		if v2ListContainsIPv4(text, destinationIPv4) {
			ipMatches[item.Name] = true
		}
	}
	out := []benchStrategyProfile{}
	for _, profile := range inventory.Profiles {
		if !v2ProductionSourceProfileEligible(profile) {
			continue
		}
		if v2ProfileMatchesTargetWithLists(target, profile, domainMatches, ipMatches) {
			out = append(out, profile)
		}
	}
	return out
}
