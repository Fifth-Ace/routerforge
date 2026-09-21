package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const v2GenericCandidateApplyVersion = 1

func v2GenericWinnerApplyEligible(best *v2CandidateResult) error {
	if best == nil {
		return errors.New("live winner is required")
	}
	if !best.CleanupProven {
		return errors.New("live winner cleanup proof is missing")
	}
	if !best.InfrastructureOK {
		return errors.New("live winner infrastructure proof is missing")
	}
	if best.ResultClass != "WORKING" || best.SuccessRate != 1 {
		return errors.New("live winner must be repeatedly verified as WORKING")
	}
	if len(best.Args) == 0 {
		return errors.New("live winner args are empty")
	}
	if _, err := v2CustomProfile(v2PortableCandidateArgs(best.Args), "example.com"); err != nil {
		return errors.New("live winner technique does not compile: " + err.Error())
	}
	if v2CandidateTechniqueFingerprint(best.Args) == "" {
		return errors.New("live winner technique fingerprint is empty")
	}
	return nil
}

func v2StoreGenericCandidateApplyPlan(
	configSHA, serverName, destinationIPv4, transport, sessionID string,
	sourceProfile benchStrategyProfile, boundArgs []string, best *v2CandidateResult,
) (*benchAutoTuneApplyPlan, error) {
	if err := v2GenericWinnerApplyEligible(best); err != nil {
		return nil, err
	}
	target, err := v2NormalizeTarget(serverName)
	if err != nil {
		return nil, err
	}
	if _, err := v2CustomProfile(v2PortableCandidateArgs(best.Args), target); err != nil {
		return nil, errors.New("live winner technique no longer compiles: " + err.Error())
	}
	plan, err := storeBenchAutoTuneApplyPlanForCandidate(
		configSHA, target, destinationIPv4, sourceProfile, boundArgs,
		best.CandidateSource, v2CandidateTechniqueFingerprint(best.Args),
	)
	if err != nil {
		return nil, err
	}
	benchAutoTuneApplyGateState.Lock()
	if benchAutoTuneApplyGateState.plan != plan {
		benchAutoTuneApplyGateState.Unlock()
		return nil, errors.New("generic candidate apply plan changed during creation")
	}
	plan.Transport = strings.ToLower(strings.TrimSpace(transport))
	plan.SessionID = strings.TrimSpace(sessionID)
	plan.CandidateID = strings.TrimSpace(best.CandidateID)
	plan.CandidateName = strings.TrimSpace(best.CandidateName)
	plan.LiveResultClass = best.ResultClass
	plan.LiveSuccessRate = best.SuccessRate
	plan.LiveCompleteRate = best.CompleteRate
	plan.LiveCleanupProven = best.CleanupProven
	plan.LiveInfrastructureOK = best.InfrastructureOK
	benchAutoTuneApplyGateState.Unlock()
	return plan, nil
}

func v2SnapshotCandidateDependencies(config string) (map[string]string, map[string]string, []string, error) {
	listRefs, blobRefs := smartApplyReferencedNames(config)
	lists := map[string]string{}
	blobs := map[string]string{}
	resources := []string{}

	listNames := make([]string, 0, len(listRefs))
	for name := range listRefs {
		listNames = append(listNames, name)
	}
	sort.Strings(listNames)
	for _, name := range listNames {
		path, err := listSourcePath(name)
		if err != nil {
			return nil, nil, nil, err
		}
		hash, ok, err := smartApplyExistingHash(path, listFileMaxBytes)
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok {
			return nil, nil, nil, fmt.Errorf("required list is missing: %s", name)
		}
		lists[name] = hash
		resources = append(resources, "list:"+name+"@"+hash)
	}

	blobNames := make([]string, 0, len(blobRefs))
	for name := range blobRefs {
		blobNames = append(blobNames, name)
	}
	sort.Strings(blobNames)
	for _, name := range blobNames {
		path, err := blobPath(name)
		if err != nil {
			return nil, nil, nil, err
		}
		hash, ok, err := smartApplyExistingHash(path, blobMaxBytes)
		if err != nil {
			return nil, nil, nil, err
		}
		if !ok {
			return nil, nil, nil, fmt.Errorf("required blob is missing: %s", name)
		}
		blobs[name] = hash
		resources = append(resources, "blob:"+name+"@"+hash)
	}
	return lists, blobs, resources, nil
}

func v2CopyStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
