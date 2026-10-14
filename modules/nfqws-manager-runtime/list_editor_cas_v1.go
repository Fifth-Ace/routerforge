package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// NF5A: read a real, bounded list file without following a symlink. This
// protects preview and backup from a replaced list path.
func nf5aReadRegularList(path string) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, errors.New("list path is not a regular file")
	}
	if info.Size() > listFileMaxBytes {
		return nil, nil, fmt.Errorf("list file exceeds %d bytes", listFileMaxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) > listFileMaxBytes {
		return nil, nil, errors.New("list grew beyond size limit while reading")
	}
	return data, info, nil
}

// All editor saves and cross-list cleanup writes must carry the SHA returned
// by the exact /list read which produced their edit base. An absent or stale
// identity never authorizes a write, even if a backup could be created.
func nf5aRequireListPreimage(data []byte, expected string) error {
	expected = strings.TrimSpace(expected)
	if !listSourceHashPattern.MatchString(expected) {
		return errors.New("expected_sha256 must be a full SHA256 from /list")
	}
	if !strings.EqualFold(listSourceSHA256(data), expected) {
		return errors.New("list changed since it was read; reload before saving")
	}
	return nil
}
