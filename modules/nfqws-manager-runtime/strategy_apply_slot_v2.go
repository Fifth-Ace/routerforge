package main

import (
	"errors"
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

func v2ProfileMatchesTargetWithLists(target string, profile benchStrategyProfile, matchedLists map[string]bool) bool {
	matched := false
	hasSelector := false
	for _, domain := range profile.HostlistDomains {
		hasSelector = true
		if v2DomainMatches(target, domain) {
			matched = true
		}
	}
	for _, arg := range profile.FileBoundFilters {
		name, _ := v2ProfileFileRelation(arg)
		if name == "" {
			continue
		}
		hasSelector = true
		if matchedLists[name] {
			matched = true
		}
	}
	if !hasSelector {
		return true
	}
	return matched
}

func v2ProductionProfilesMatchingTarget(target string, inventory benchStrategyInventory) []benchStrategyProfile {
	matchedLists := map[string]bool{}
	for _, item := range readStatus().Lists {
		data, _, err := v2ReadListByName(item.Name)
		if err == nil && v2ListContainsTarget(string(data), target) {
			matchedLists[item.Name] = true
		}
	}
	out := []benchStrategyProfile{}
	for _, profile := range inventory.Profiles {
		if !v2ProductionSourceProfileEligible(profile) {
			continue
		}
		if v2ProfileMatchesTargetWithLists(target, profile, matchedLists) {
			out = append(out, profile)
		}
	}
	return out
}
