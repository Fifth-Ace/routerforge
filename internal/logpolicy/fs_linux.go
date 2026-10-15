//go:build linux

package logpolicy

import (
	"os"
	"syscall"
)

func ramFSAvailable() bool                   { return isRAMFS("/tmp") }
func ramDirectoryAvailable(path string) bool { return isRAMFS(path) }
func isRAMFS(path string) bool {
	var fs syscall.Statfs_t
	if syscall.Statfs(path, &fs) != nil {
		return false
	}
	t := uint64(fs.Type)
	return t == 0x01021994 || t == 0x858458f6
}
func openAppendNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
