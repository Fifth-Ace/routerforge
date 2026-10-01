package main

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	antiscanSetPageDefaultLimit = 200
	antiscanSetPageMaxLimit     = 500
	antiscanSetListTimeout      = 5 * time.Second
)

type antiscanSetEntry struct {
	Value          string `json:"value"`
	TimeoutSeconds int64  `json:"timeout_seconds,omitempty"`
	TimeoutKnown   bool   `json:"timeout_known"`
	Packets        int64  `json:"packets,omitempty"`
	PacketsKnown   bool   `json:"packets_known"`
	Bytes          int64  `json:"bytes,omitempty"`
	BytesKnown     bool   `json:"bytes_known"`
}

type antiscanSetPage struct {
	GeneratedAt time.Time          `json:"generated_at"`
	Name        string             `json:"name"`
	Detected    bool               `json:"detected"`
	Running     bool               `json:"running"`
	Exists      bool               `json:"exists"`
	Count       int64              `json:"count,omitempty"`
	CountKnown  bool               `json:"count_known"`
	Limit       int                `json:"limit"`
	Truncated   bool               `json:"truncated"`
	Entries     []antiscanSetEntry `json:"entries"`
	MutationAPI bool               `json:"mutation_api"`
	Error       string             `json:"error,omitempty"`
}

func parseAntiscanSetLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return antiscanSetPageDefaultLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > antiscanSetPageMaxLimit {
		return 0, errors.New("limit must be between 1 and 500")
	}
	return value, nil
}

func browseAntiscanSet(parent context.Context, cfg runtimeConfig, name string, limit int) (antiscanSetPage, int) {
	page := antiscanSetPage{
		GeneratedAt: time.Now().UTC(),
		Name:        name,
		Limit:       limit,
		Entries:     []antiscanSetEntry{},
		MutationAPI: true,
	}
	if !knownAntiscanSet(name) {
		page.Error = "unknown Antiscan ipset"
		return page, 400
	}

	configPath := cfg.AntiscanDir + "/ascn.conf"
	page.Detected = pathExists(cfg.InitScript) || pathExists(configPath) || readAntiscanVersion(cfg) != ""
	page.Running = pathExists(cfg.StatusFile)
	if !page.Detected {
		return page, 200
	}

	binary := findAntiscanIPSetBinary()
	if binary == "" {
		page.Error = "ipset binary not found"
		return page, 200
	}

	exists, err := antiscanSetExists(parent, binary, name)
	if err != nil {
		page.Error = err.Error()
		return page, 200
	}
	page.Exists = exists
	if !exists {
		return page, 200
	}

	entries, count, countKnown, truncated, err := readAntiscanSetEntries(parent, binary, name, limit)
	page.Entries = entries
	page.Count = count
	page.CountKnown = countKnown
	page.Truncated = truncated
	if err != nil {
		page.Error = err.Error()
	}
	return page, 200
}

func antiscanSetExists(parent context.Context, binary, setName string) (bool, error) {
	if !knownAntiscanSet(setName) {
		return false, errors.New("unknown Antiscan ipset")
	}
	ctx, cancel := context.WithTimeout(parent, antiscanCommandTimeout)
	defer cancel()
	data, err := safety.RunCommand(ctx, antiscanCommandOutputMax, binary, "-n", "list")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == setName {
			return true, nil
		}
	}
	return false, nil
}

func readAntiscanSetEntries(parent context.Context, binary, setName string, limit int) ([]antiscanSetEntry, int64, bool, bool, error) {
	if !knownAntiscanSet(setName) {
		return nil, 0, false, false, errors.New("unknown Antiscan ipset")
	}
	if limit < 1 || limit > antiscanSetPageMaxLimit {
		return nil, 0, false, false, errors.New("invalid Antiscan set page limit")
	}

	ctx, cancel := context.WithTimeout(parent, antiscanSetListTimeout)
	defer cancel()
	cmd, err := safety.CommandContext(ctx, binary, "list", setName)
	if err != nil {
		return nil, 0, false, false, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, false, false, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, 0, false, false, err
	}

	entries, count, countKnown, truncated, parseErr := parseAntiscanSetList(stdout, limit)
	waitErr := cmd.Wait()
	if parseErr != nil {
		return entries, count, countKnown, truncated, parseErr
	}
	if ctx.Err() != nil {
		return entries, count, countKnown, true, ctx.Err()
	}
	if waitErr != nil {
		return entries, count, countKnown, truncated, waitErr
	}
	return entries, count, countKnown, truncated, nil
}

func parseAntiscanSetList(r io.Reader, limit int) ([]antiscanSetEntry, int64, bool, bool, error) {
	if limit < 1 {
		return nil, 0, false, false, errors.New("invalid Antiscan set page limit")
	}

	entries := make([]antiscanSetEntry, 0, limit)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	inMembers := false
	var count int64
	countKnown := false
	seen := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !inMembers {
			if strings.HasPrefix(line, "Number of entries:") {
				raw := strings.TrimSpace(strings.TrimPrefix(line, "Number of entries:"))
				value, err := strconv.ParseInt(raw, 10, 64)
				if err == nil && value >= 0 {
					count = value
					countKnown = true
				}
			}
			if line == "Members:" {
				inMembers = true
			}
			continue
		}
		if line == "" {
			continue
		}
		entry, ok := parseAntiscanSetEntry(line)
		if !ok {
			continue
		}
		seen++
		if len(entries) < limit {
			entries = append(entries, entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return entries, count, countKnown, seen > len(entries), err
	}
	truncated := seen > len(entries)
	if countKnown && count > int64(len(entries)) {
		truncated = true
	}
	return entries, count, countKnown, truncated, nil
}

func parseAntiscanSetEntry(line string) (antiscanSetEntry, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return antiscanSetEntry{}, false
	}
	value := fields[0]
	if _, err := netip.ParseAddr(value); err != nil {
		if _, prefixErr := netip.ParsePrefix(value); prefixErr != nil {
			return antiscanSetEntry{}, false
		}
	}
	entry := antiscanSetEntry{Value: value}
	for i := 1; i+1 < len(fields); i++ {
		switch fields[i] {
		case "timeout":
			if n, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil && n >= 0 {
				entry.TimeoutSeconds = n
				entry.TimeoutKnown = true
			}
		case "packets":
			if n, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil && n >= 0 {
				entry.Packets = n
				entry.PacketsKnown = true
			}
		case "bytes":
			if n, err := strconv.ParseInt(fields[i+1], 10, 64); err == nil && n >= 0 {
				entry.Bytes = n
				entry.BytesKnown = true
			}
		}
	}
	return entry, true
}
