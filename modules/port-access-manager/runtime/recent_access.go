package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const recentRoot = "/proc/net/xt_recent"
const recentAuthName = "RF_KNOCK_AUTH"

type recentClient struct {
	IP           string `json:"ip"`
	LastSeenUnix int64  `json:"last_seen_unix,omitempty"`
	TTLSeconds   int64  `json:"ttl_seconds,omitempty"`
	TTLKnown     bool   `json:"ttl_known"`
}

func readRecentClients(path string, ttl int, now time.Time) ([]recentClient, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1<<20)
	out := make([]recentClient, 0)
	for s.Scan() {
		if len(out) >= 2048 {
			return nil, errors.New("recent list too large")
		}
		fields := strings.Fields(s.Text())
		if len(fields) == 0 {
			continue
		}
		if !strings.HasPrefix(fields[0], "src=") {
			continue
		}
		ip := strings.TrimPrefix(fields[0], "src=")
		addr := net.ParseIP(ip)
		if addr == nil {
			continue
		}
		c := recentClient{IP: addr.String()}
		for _, token := range fields[1:] {
			if strings.HasPrefix(token, "last_seen:") {
				value := strings.TrimPrefix(token, "last_seen:")
				// xt_recent reports kernel jiffies, not Unix seconds. Do not invent wall-clock ages.
				_, _ = strconv.ParseUint(value, 10, 64)
			}
		}
		out = append(out, c)
	}
	return out, s.Err()
}

func recentAccessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		jsonReply(w, 405, map[string]string{"error": "GET required"})
		return
	}
	path := filepath.Join(recentRoot, recentAuthName)
	entries, err := readRecentClients(path, 0, time.Now())
	if err != nil {
		if os.IsNotExist(err) {
			jsonReply(w, 200, map[string]any{"available": false, "entries": []recentClient{}, "reason": "recent authorization list is not active"})
			return
		}
		jsonReply(w, 503, map[string]string{"error": "unable to read recent authorization list"})
		return
	}
	jsonReply(w, 200, map[string]any{"available": true, "engine": "iptables-recent", "list": recentAuthName, "entries": entries, "note": "kernel current state; timeout cannot be inferred from jiffies alone"})
}

func recentRevokeHandler(w http.ResponseWriter, r *http.Request) {
	// Activation of this action depends on the established Core guarded mutation gate.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		jsonReply(w, 405, map[string]string{"error": "POST required"})
		return
	}
	if r.Header.Get("X-RouterForge-Module-Authorized") != "core-authorized-v1" {
		jsonReply(w, 403, map[string]string{"error": "Core authorization required"})
		return
	}
	if r.ContentLength > 512 {
		jsonReply(w, 413, map[string]string{"error": "request too large"})
		return
	}
	var req struct {
		IP      string `json:"ip"`
		Confirm string `json:"confirm"`
	}
	if err := decodeRecentRevoke(r, &req); err != nil {
		jsonReply(w, 400, map[string]string{"error": "invalid revoke request"})
		return
	}
	ip := net.ParseIP(req.IP)
	if ip == nil || req.Confirm != "REVOKE" || strings.Contains(req.IP, "%") {
		jsonReply(w, 400, map[string]string{"error": "IP and confirmation required"})
		return
	}
	path := filepath.Join(recentRoot, recentAuthName)
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		jsonReply(w, 409, map[string]string{"error": "authorization list unavailable"})
		return
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		jsonReply(w, 503, map[string]string{"error": "recent list not writable"})
		return
	}
	defer f.Close()
	if _, err = fmt.Fprintf(f, "-%s\n", ip.String()); err != nil {
		jsonReply(w, 503, map[string]string{"error": "kernel revoke failed"})
		return
	}
	jsonReply(w, 200, map[string]any{"ok": true, "ip": ip.String(), "action": "revoke"})
}

func decodeRecentRevoke(r *http.Request, target any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 513))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return errors.New("trailing content")
	}
	return nil
}
