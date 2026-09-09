//go:build !linux

package main

import "errors"

type filesystemStats struct {
	Total     uint64
	Free      uint64
	Available uint64
}

func readFilesystemStats(string) (filesystemStats, error) {
	return filesystemStats{}, errors.New("filesystem stats unsupported on this platform")
}
