//go:build linux

package main

import "syscall"

func readPlatformStorage(mount string) platformStorage {
	out := platformStorage{Mount: mount}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(mount, &stat); err != nil {
		return out
	}
	blockSize := uint64(stat.Bsize)
	out.TotalBytes = stat.Blocks * blockSize
	out.FreeBytes = stat.Bfree * blockSize
	out.AvailableBytes = stat.Bavail * blockSize
	if out.TotalBytes >= out.FreeBytes {
		out.UsedBytes = out.TotalBytes - out.FreeBytes
	}
	if out.TotalBytes > 0 {
		out.UsedPct = float64(out.UsedBytes) / float64(out.TotalBytes) * 100
	}
	return out
}
