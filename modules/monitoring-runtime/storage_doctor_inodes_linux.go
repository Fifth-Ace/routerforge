//go:build linux

package main

import "syscall"

// Linux-only filesystem inode accounting. Reads metadata; does not mutate the mount.
func doctorFSInodes(path string) (*uint64, *uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return nil, nil
	}
	total := uint64(stat.Files)
	free := uint64(stat.Ffree)
	return &free, &total
}
