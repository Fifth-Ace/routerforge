package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func coreLogEvent(originalPath, kind, message string) {
	if originalPath == "" {
		return
	}
	coreLoggingMu.Lock()
	defer coreLoggingMu.Unlock()
	s, err := readCoreLoggingSettings()
	if err != nil {
		return // Never silently redirect a damaged configuration to flash.
	}
	path, err := resolveCoreLogPath(s, originalPath, coreRAMLoggingAvailable())
	if err != nil {
		return // RAM was requested but is not actually backed by tmpfs.
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	limit := int64(s.MaxMB) * 1024 * 1024
	if st, err := os.Stat(path); err == nil && st.Size() >= limit {
		for i := s.Backups; i >= 1; i-- {
			old := fmt.Sprintf("%s.%d", path, i)
			if i == s.Backups {
				_ = os.Remove(old)
			} else {
				_ = os.Rename(old, fmt.Sprintf("%s.%d", path, i+1))
			}
		}
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %-16s %s\n", time.Now().Format("2006-01-02 15:04:05"), kind, message)
}
