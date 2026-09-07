package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const rfWebListenState = "0A"

func rfDiscoverLocalTCPListeners(procRoot, opkgInfoDir string) ([]rfWebListener, error) {
	listeners, err := rfReadLocalTCPListeners(procRoot)
	if err != nil {
		return nil, err
	}
	return rfAttachLocalTCPListenerOwners(procRoot, opkgInfoDir, listeners)
}

type rfWebListenerOwner struct {
	PID        int      `json:"pid"`
	Process    string   `json:"process,omitempty"`
	Executable string   `json:"executable,omitempty"`
	Command    []string `json:"command,omitempty"`
	Package    string   `json:"package,omitempty"`
}

type rfWebListener struct {
	Address string               `json:"address"`
	Port    int                  `json:"port"`
	Inode   uint64               `json:"inode"`
	Owners  []rfWebListenerOwner `json:"owners,omitempty"`
}

func rfReadLocalTCPListeners(procRoot string) ([]rfWebListener, error) {
	var listeners []rfWebListener
	found := false
	for _, spec := range []struct {
		name string
		ipv6 bool
	}{{"tcp", false}, {"tcp6", true}} {
		path := filepath.Join(procRoot, "net", spec.name)
		file, err := os.Open(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		found = true
		parsed, parseErr := rfParseProcNetTCP(file, spec.ipv6)
		closeErr := file.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", path, parseErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		listeners = append(listeners, parsed...)
	}
	if !found {
		return nil, os.ErrNotExist
	}
	sort.Slice(listeners, func(i, j int) bool {
		if listeners[i].Port != listeners[j].Port {
			return listeners[i].Port < listeners[j].Port
		}
		if listeners[i].Address != listeners[j].Address {
			return listeners[i].Address < listeners[j].Address
		}
		return listeners[i].Inode < listeners[j].Inode
	})
	return listeners, nil
}

func rfParseProcNetTCP(reader io.Reader, ipv6 bool) ([]rfWebListener, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	var listeners []rfWebListener
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 || (lineNo == 1 && strings.EqualFold(fields[0], "sl")) {
			continue
		}
		if len(fields) < 10 {
			return nil, fmt.Errorf("line %d: expected at least 10 fields", lineNo)
		}
		if fields[3] != rfWebListenState {
			continue
		}
		address, port, err := rfDecodeProcTCPAddress(fields[1], ipv6)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		inode, err := strconv.ParseUint(fields[9], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid inode: %w", lineNo, err)
		}
		if inode == 0 {
			continue
		}
		listeners = append(listeners, rfWebListener{Address: address, Port: port, Inode: inode})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return listeners, nil
}

func rfDecodeProcTCPAddress(value string, ipv6 bool) (string, int, error) {
	hostHex, portHex, ok := strings.Cut(value, ":")
	if !ok {
		return "", 0, fmt.Errorf("invalid local address %q", value)
	}
	portValue, err := strconv.ParseUint(portHex, 16, 16)
	if err != nil || portValue == 0 {
		if err == nil {
			err = errors.New("zero port")
		}
		return "", 0, fmt.Errorf("invalid local port %q: %w", portHex, err)
	}
	raw, err := hex.DecodeString(hostHex)
	if err != nil {
		return "", 0, fmt.Errorf("invalid local host %q: %w", hostHex, err)
	}
	if ipv6 {
		if len(raw) != net.IPv6len {
			return "", 0, fmt.Errorf("invalid IPv6 host length %d", len(raw))
		}
		for i := 0; i < len(raw); i += 4 {
			raw[i], raw[i+3] = raw[i+3], raw[i]
			raw[i+1], raw[i+2] = raw[i+2], raw[i+1]
		}
		return net.IP(raw).String(), int(portValue), nil
	}
	if len(raw) != net.IPv4len {
		return "", 0, fmt.Errorf("invalid IPv4 host length %d", len(raw))
	}
	raw[0], raw[3] = raw[3], raw[0]
	raw[1], raw[2] = raw[2], raw[1]
	return net.IP(raw).String(), int(portValue), nil
}

func rfAttachLocalTCPListenerOwners(procRoot, opkgInfoDir string, listeners []rfWebListener) ([]rfWebListener, error) {
	if len(listeners) == 0 {
		return listeners, nil
	}
	wanted := make(map[uint64]struct{}, len(listeners))
	for _, listener := range listeners {
		wanted[listener.Inode] = struct{}{}
	}
	packagePaths, err := rfReadOpkgFileOwners(opkgInfoDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	owners, err := rfReadSocketOwners(procRoot, wanted, packagePaths)
	if err != nil {
		return nil, err
	}
	for i := range listeners {
		listeners[i].Owners = append([]rfWebListenerOwner(nil), owners[listeners[i].Inode]...)
	}
	return listeners, nil
}

func rfReadSocketOwners(procRoot string, wanted map[uint64]struct{}, packagePaths map[string]string) (map[uint64][]rfWebListenerOwner, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	result := make(map[uint64][]rfWebListenerOwner)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		pidRoot := filepath.Join(procRoot, entry.Name())
		fdEntries, err := os.ReadDir(filepath.Join(pidRoot, "fd"))
		if err != nil {
			continue
		}
		matched := make(map[uint64]struct{})
		for _, fd := range fdEntries {
			target, err := os.Readlink(filepath.Join(pidRoot, "fd", fd.Name()))
			if err != nil {
				continue
			}
			inode, ok := rfSocketInode(target)
			if !ok {
				continue
			}
			if _, ok := wanted[inode]; !ok {
				continue
			}
			matched[inode] = struct{}{}
		}
		if len(matched) == 0 {
			continue
		}
		exe, _ := os.Readlink(filepath.Join(pidRoot, "exe"))
		exe = strings.TrimSuffix(exe, " (deleted)")
		command := rfReadProcCommand(filepath.Join(pidRoot, "cmdline"))
		process := rfOwnerProcessName(exe, command)
		pkg := rfPackageForOwner(exe, command, packagePaths)
		owner := rfWebListenerOwner{PID: pid, Process: process, Executable: exe, Command: command, Package: pkg}
		for inode := range matched {
			result[inode] = append(result[inode], owner)
		}
	}
	for inode := range result {
		sort.Slice(result[inode], func(i, j int) bool {
			return result[inode][i].PID < result[inode][j].PID
		})
	}
	return result, nil
}

func rfSocketInode(target string) (uint64, bool) {
	if !strings.HasPrefix(target, "socket:[") || !strings.HasSuffix(target, "]") {
		return 0, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
	inode, err := strconv.ParseUint(value, 10, 64)
	return inode, err == nil && inode != 0
}

func rfReadProcCommand(path string) []string {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return nil
	}
	parts := strings.Split(string(raw), "\x00")
	command := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			command = append(command, part)
		}
	}
	return command
}

func rfOwnerProcessName(exe string, command []string) string {
	if len(command) > 0 {
		if base := filepath.Base(command[0]); base != "." && base != string(filepath.Separator) {
			return base
		}
	}
	return filepath.Base(exe)
}

func rfReadOpkgFileOwners(infoDir string) (map[string]string, error) {
	entries, err := os.ReadDir(infoDir)
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".list") {
			continue
		}
		pkg := strings.TrimSuffix(entry.Name(), ".list")
		file, err := os.Open(filepath.Join(infoDir, entry.Name()))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 512*1024)
		for scanner.Scan() {
			path := strings.TrimSpace(scanner.Text())
			if path == "" || !filepath.IsAbs(path) {
				continue
			}
			if _, exists := owners[path]; !exists {
				owners[path] = pkg
			}
		}
		_ = file.Close()
	}
	return owners, nil
}

func rfPackageForOwner(exe string, command []string, packagePaths map[string]string) string {
	candidates := make([]string, 0, len(command)+1)
	if len(command) > 1 {
		candidates = append(candidates, command[1:]...)
	}
	candidates = append(candidates, exe)
	if len(command) > 0 {
		candidates = append(candidates, command[0])
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSuffix(candidate, " (deleted)")
		if !filepath.IsAbs(candidate) {
			continue
		}
		if pkg := packagePaths[candidate]; pkg != "" {
			return pkg
		}
	}
	return ""
}
