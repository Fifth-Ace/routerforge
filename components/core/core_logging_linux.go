//go:build linux

package main

import "syscall"

func coreRAMLoggingAvailable() bool {
	var fs syscall.Statfs_t
	if syscall.Statfs("/tmp", &fs) != nil {
		return false
	}
	magic := uint64(fs.Type)
	return magic == 0x01021994 || magic == 0x858458f6
}
