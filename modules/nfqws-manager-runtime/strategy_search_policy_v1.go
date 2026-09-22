package main

import "time"

const (
	v2DirectSelectorMaxCandidates = 512
	v2DirectNormalBatchSize       = 20
	v2DirectNormalWorkingTarget   = 2
	v2DirectFastPercent           = 30
)

func v2DirectAutoPoolLimit(mode string, corpusCount int) int {
	if corpusCount <= 0 {
		return 0
	}
	switch mode {
	case "fast":
		limit := (corpusCount*v2DirectFastPercent + 99) / 100
		if limit < 1 {
			limit = 1
		}
		if limit > corpusCount {
			limit = corpusCount
		}
		return limit
	case "normal", "thorough":
		return corpusCount
	default:
		return corpusCount
	}
}

func v2DirectBatchSize(mode string, remaining int) int {
	if remaining <= 0 {
		return 0
	}
	if mode == "normal" && remaining > v2DirectNormalBatchSize {
		return v2DirectNormalBatchSize
	}
	return remaining
}

func v2DirectShouldStopAfterBatch(mode string, working, next, total int) bool {
	if next >= total {
		return true
	}
	switch mode {
	case "fast":
		return true
	case "normal":
		return working >= v2DirectNormalWorkingTarget
	case "thorough":
		return false
	default:
		return true
	}
}

func v2DirectBenchTimeout(mode benchAutoTuneMode, planned, concurrency int) time.Duration {
	if concurrency < 1 {
		concurrency = 1
	}
	if planned < 1 {
		planned = 1
	}
	waves := (planned + concurrency - 1) / concurrency
	// Each real HTTPS probe can consume up to ~20s; startup/cleanup adds margin.
	direct := time.Duration(waves*25+60) * time.Second
	legacy := v2SelectorBenchTimeout(mode, concurrency)
	if direct < legacy {
		return legacy
	}
	return direct
}
