//go:build !linux

package logpolicy

import "os"

func ramFSAvailable() bool              { return false }
func ramDirectoryAvailable(string) bool { return false }
func openAppendNoFollow(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
}
