//go:build linux

package main

import "syscall"

type filesystemStats struct {
	Total     uint64
	Free      uint64
	Available uint64
}

func readFilesystemStats(path string) (filesystemStats, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return filesystemStats{}, err
	}

	blockSize := uint64(stat.Bsize)
	return filesystemStats{
		Total:     uint64(stat.Blocks) * blockSize,
		Free:      uint64(stat.Bfree) * blockSize,
		Available: uint64(stat.Bavail) * blockSize,
	}, nil
}
