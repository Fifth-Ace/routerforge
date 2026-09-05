package main

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type systemInfo struct {
	PID        int    `json:"pid"`
	GoVersion  string `json:"go_version"`
	GOARCH     string `json:"goarch"`
	GOOS       string `json:"goos"`
	Goroutines int    `json:"goroutines"`
	RSSKB      int64  `json:"rss_kb"`
	VmSizeKB   int64  `json:"vmsize_kb"`
}

func readSystemInfo() systemInfo {
	out := systemInfo{PID: os.Getpid(), GoVersion: runtime.Version(), GOARCH: runtime.GOARCH, GOOS: runtime.GOOS, Goroutines: runtime.NumGoroutine()}
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			out.RSSKB = parseStatusKB(line)
		}
		if strings.HasPrefix(line, "VmSize:") {
			out.VmSizeKB = parseStatusKB(line)
		}
	}
	return out
}

func parseStatusKB(line string) int64 {
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0
	}
	n, _ := strconv.ParseInt(f[1], 10, 64)
	return n
}
