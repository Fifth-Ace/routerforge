package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func coreLogEvent(path, kind, message string) {
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	if st, err := os.Stat(path); err == nil && st.Size() >= 1024*1024 {
		_ = os.Remove(path + ".1")
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %-16s %s\n", time.Now().Format("2006-01-02 15:04:05"), kind, message)
}
