package main

const (
	v2ProgressiveStageMemory = "MEMORY"
	v2ProgressiveStageQuick  = "QUICK"
	v2ProgressiveStageFull   = "FULL"
)

func v2ProgressiveQuickLimit(mode benchAutoTuneMode) int {
	switch mode.Name {
	case "fast":
		return 2
	case "thorough":
		return 4
	default:
		return 3
	}
}
