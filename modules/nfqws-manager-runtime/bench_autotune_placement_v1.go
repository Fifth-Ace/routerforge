package main

import "errors"

// nf4BuildPlacementConfig is shared by Preview and Apply. The placement mode
// comes from the verified plan, not from the later client request.
func nf4BuildPlacementConfig(config string, appendProfile bool, sourceArgs, candidateArgs []string) (string, error) {
	if appendProfile {
		if len(sourceArgs) != 0 {
			return "", errors.New("append mode must not carry a source profile")
		}
		return buildAutoTuneAppendProfileConfig(config, candidateArgs)
	}
	if len(sourceArgs) == 0 {
		return "", errors.New("replacement mode requires an exact source profile")
	}
	return buildAutoTuneCandidateConfig(config, sourceArgs, candidateArgs)
}
