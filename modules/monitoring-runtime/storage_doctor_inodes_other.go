//go:build !linux

package main

// On non-Linux CI targets, Linux inode counters are unavailable. Do not invent values.
func doctorFSInodes(path string) (*uint64, *uint64) {
	return nil, nil
}
