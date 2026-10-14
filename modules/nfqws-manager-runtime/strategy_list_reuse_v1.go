package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// NF4B reuses only one identical, exclusively owned, file-backed HTTPS profile.
// An ambiguous match is never allowed to mutate an existing list.
func nf4bEligibleListProfile(profile benchStrategyProfile, fingerprint string) (string, bool) {
	if !v2ProductionSourceProfileEligible(profile) ||
		v2CandidateTechniqueFingerprint(profile.Args) != fingerprint || fingerprint == "" {
		return "", false
	}
	name := ""
	for _, arg := range profile.Args {
		if !v2StrategySelectionArg(arg) {
			continue
		}
		if !strings.HasPrefix(arg, "--hostlist=") || name != "" {
			return "", false
		}
		path := strings.TrimPrefix(arg, "--hostlist=")
		candidate := filepath.Base(path)
		exact, err := listSourcePath(candidate)
		if err != nil || filepath.Clean(path) != exact || path != exact {
			return "", false
		}
		name = candidate
	}
	return name, name != ""
}

func nf4bAppendDomain(content []byte, target string) ([]byte, error) {
	if len(content) == 0 || !utf8.Valid(content) || len(content) >= int(listFileMaxBytes)-512 {
		return nil, errors.New("list content is invalid or exceeds safe append budget")
	}
	if err := validateListContent(string(content)); err != nil {
		return nil, err
	}
	if v2ListContainsTarget(string(content), target) {
		return nil, errors.New("target already exists in this list")
	}
	newline := "\n"
	if strings.Contains(string(content), "\r\n") {
		newline = "\r\n"
	}
	out := append([]byte(nil), content...)
	if !strings.HasSuffix(string(out), "\n") {
		out = append(out, []byte(newline)...)
	}
	out = append(out, []byte(target+newline)...)
	if len(out) > int(listFileMaxBytes) {
		return nil, errors.New("updated list exceeds size limit")
	}
	if err := validateListContent(string(out)); err != nil {
		return nil, err
	}
	return out, nil
}

// All shared-list, routing and fingerprint checks are performed again for
// Preview and Apply; earlier selector metadata is not trusted for mutation.
func nf4bFindReusableList(target, ip, fingerprint string, inventory benchStrategyInventory) (benchStrategyProfile, string, string, []byte, error) {
	normalized, err := v2NormalizeTarget(target)
	if err != nil || normalized != target {
		return benchStrategyProfile{}, "", "", nil, errors.New("invalid normalized list target")
	}
	if len(v2ProductionProfilesMatchingTarget(target, ip, inventory)) != 0 {
		return benchStrategyProfile{}, "", "", nil, errors.New("target already binds production profile; reuse is ambiguous")
	}
	var matched *benchStrategyProfile
	listName := ""
	for i := range inventory.Profiles {
		p := inventory.Profiles[i]
		name, ok := nf4bEligibleListProfile(p, fingerprint)
		if !ok {
			continue
		}
		if matched != nil {
			return benchStrategyProfile{}, "", "", nil, errors.New("multiple profiles have identical technique")
		}
		copyProfile := p
		matched = &copyProfile
		listName = name
	}
	if matched == nil {
		return benchStrategyProfile{}, "", "", nil, errors.New("no exclusive reusable hostlist profile")
	}
	// Count all uses, including exclusions/ipsets, to prevent a change that
	// alters the behavior of another production profile.
	count := 0
	for _, profile := range inventory.Profiles {
		for _, arg := range profile.Args {
			name, _ := v2ProfileFileRelation(arg)
			if name == listName {
				count++
			}
		}
	}
	if count != 1 {
		return benchStrategyProfile{}, "", "", nil, errors.New("hostlist is shared with another profile")
	}
	path, err := listSourcePath(listName)
	if err != nil {
		return benchStrategyProfile{}, "", "", nil, err
	}
	if _, err = regularListSourceFile(path); err != nil {
		return benchStrategyProfile{}, "", "", nil, fmt.Errorf("list is not a safe regular file: %w", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return benchStrategyProfile{}, "", "", nil, err
	}
	updated, err := nf4bAppendDomain(original, target)
	if err != nil {
		return benchStrategyProfile{}, "", "", nil, err
	}
	return *matched, listName, listSourceSHA256(original), updated, nil
}

func nf4bSetListPlan(plan *benchAutoTuneApplyPlan, profile benchStrategyProfile, name, oldSHA string) error {
	benchAutoTuneApplyGateState.Lock()
	defer benchAutoTuneApplyGateState.Unlock()
	if plan == nil || benchAutoTuneApplyGateState.plan != plan || name == "" || !smartApplyHashPattern.MatchString(oldSHA) {
		return errors.New("list placement plan identity changed")
	}
	plan.AppendProfile = false
	plan.SourceProfileIndex = profile.Index
	plan.SourceStrategyArgs = append([]string(nil), profile.Args...)
	plan.ListAppendName = name
	plan.ListAppendSHA = oldSHA
	return nil
}

func nf4bStoreReceiptList(plan *benchAutoTuneApplyPlan, updated []byte) error {
	benchAutoTuneApplyReceiptState.Lock()
	defer benchAutoTuneApplyReceiptState.Unlock()
	if plan == nil || plan.ListAppendName == "" || len(updated) == 0 ||
		benchAutoTuneApplyReceiptState.receipt == nil || benchAutoTuneApplyReceiptState.receipt.Token != plan.Token {
		return errors.New("list preview receipt identity changed")
	}
	r := benchAutoTuneApplyReceiptState.receipt
	r.ListAppendName = plan.ListAppendName
	r.ListAppendSHA = plan.ListAppendSHA
	r.ListAppendBytes = append([]byte(nil), updated...)
	return nil
}
