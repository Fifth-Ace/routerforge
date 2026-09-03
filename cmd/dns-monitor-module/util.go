package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func readBytes(path string) []byte {
	b, _ := os.ReadFile(path)
	return b
}

func readTrimmed(path string) string {
	return strings.TrimSpace(string(readBytes(path)))
}

func readFloatField(path string, index int) float64 {
	fields := strings.Fields(readTrimmed(path))
	if index < 0 || index >= len(fields) {
		return 0
	}
	n, _ := strconv.ParseFloat(fields[index], 64)
	return n
}

func readLoadAverage() [3]float64 {
	var out [3]float64
	fields := strings.Fields(readTrimmed("/proc/loadavg"))
	for i := 0; i < len(fields) && i < 3; i++ {
		out[i], _ = strconv.ParseFloat(fields[i], 64)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseUint(value string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	return n
}

func hostname() string {
	h, _ := os.Hostname()
	return h
}

func scanKeyValueFile(path string, separator byte) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		idx := strings.IndexByte(line, separator)
		if idx <= 0 {
			continue
		}
		out[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	return out
}
