//go:build linux

package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

const (
	adminTerminalWebSocketMaxClientFrame = 64 << 10
	adminTerminalWebSocketGUID           = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

type terminalWebSocketResize struct {
	Type string `json:"type"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

type terminalWebSocketWriter struct {
	mu sync.Mutex
	w  *bufio.Writer
}

func registerAdminTerminalWebSocketRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/terminal/ws", handleAdminTerminalWebSocket)
}

func handleAdminTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get(adminMutationAuthorizationHeader) != adminMutationAuthorizationValue {
		writeJSON(w, http.StatusForbidden, map[string]any{"error": "authorized RouterForge Core session required"})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
		return
	}
	if !webSocketUpgradeRequested(r) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "WebSocket upgrade required"})
		return
	}

	cwd := strings.TrimSpace(r.URL.Query().Get("cwd"))
	if cwd == "" {
		cwd = "/opt"
	}
	resolved, err := resolveExistingAdminFilePath(cwd)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	info, err := os.Stat(resolved.Canonical)
	if err != nil {
		writeAdminFilePathError(w, err)
		return
	}
	if !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "terminal cwd is not a directory"})
		return
	}

	cols := queryTerminalDimension(r, "cols", 80)
	rows := queryTerminalDimension(r, "rows", 24)
	cols, rows = normalizeTerminalSize(cols, rows)

	cmd := exec.Command("/opt/bin/sh", "-il")
	cmd.Dir = resolved.Canonical
	cmd.Env = terminalWebSocketEnvironment(os.Environ())
	master, err := startTerminalPTY(cmd, cols, rows)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": fmt.Sprintf("cannot start Entware PTY: %v", err)})
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		closeTerminalPTY(master, cmd)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "WebSocket hijacking unavailable"})
		return
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		closeTerminalPTY(master, cmd)
		return
	}

	accepted := webSocketAccept(r.Header.Get("Sec-WebSocket-Key"))
	if _, err := fmt.Fprintf(
		rw,
		"HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		accepted,
	); err != nil {
		_ = conn.Close()
		closeTerminalPTY(master, cmd)
		return
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		closeTerminalPTY(master, cmd)
		return
	}

	writer := &terminalWebSocketWriter{w: rw}
	done := make(chan struct{})
	var doneOnce sync.Once
	signalDone := func() { doneOnce.Do(func() { close(done) }) }

	go func() {
		buffer := make([]byte, 8192)
		for {
			n, readErr := master.Read(buffer)
			if n > 0 {
				if writeErr := writer.writeFrame(0x2, buffer[:n]); writeErr != nil {
					signalDone()
					_ = conn.Close()
					return
				}
			}
			if readErr != nil {
				signalDone()
				_ = conn.Close()
				return
			}
		}
	}()

	go func() {
		_ = cmd.Wait()
		_ = writer.writeFrame(0x8, []byte{0x03, 0xe8})
		signalDone()
		_ = conn.Close()
	}()

	reader := rw.Reader
	for {
		opcode, payload, err := readWebSocketClientFrame(reader)
		if err != nil {
			break
		}
		switch opcode {
		case 0x1:
			var resize terminalWebSocketResize
			if err := json.Unmarshal(payload, &resize); err != nil || resize.Type != "resize" {
				continue
			}
			nextCols, nextRows := normalizeTerminalSize(resize.Cols, resize.Rows)
			_ = setTerminalWindowSize(master.Fd(), nextCols, nextRows)
		case 0x2:
			if len(payload) != 0 {
				if _, err := master.Write(payload); err != nil {
					break
				}
			}
		case 0x8:
			_ = writer.writeFrame(0x8, payload)
			goto closed
		case 0x9:
			_ = writer.writeFrame(0xA, payload)
		case 0xA:
			// pong
		default:
			goto closed
		}
	}

closed:
	signalDone()
	_ = conn.Close()
	closeTerminalPTY(master, cmd)
	<-done
}

func webSocketUpgradeRequested(r *http.Request) bool {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		return false
	}
	connection := strings.ToLower(r.Header.Get("Connection"))
	if !strings.Contains(connection, "upgrade") {
		return false
	}
	if strings.TrimSpace(r.Header.Get("Sec-WebSocket-Version")) != "13" {
		return false
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	decoded, err := base64.StdEncoding.DecodeString(key)
	return err == nil && len(decoded) == 16
}

func webSocketAccept(key string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(key) + adminTerminalWebSocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func queryTerminalDimension(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get(name)))
	if err != nil {
		return fallback
	}
	return value
}

func terminalWebSocketEnvironment(env []string) []string {
	managed := map[string]string{
		"PATH":    "/opt/sbin:/opt/bin:/opt/usr/sbin:/opt/usr/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME":    "/opt/root",
		"USER":    "root",
		"LOGNAME": "root",
		"SHELL":   "/opt/bin/sh",
		"TERM":    "xterm-256color",
		"LANG":    "C.UTF-8",
	}
	out := make([]string, 0, len(env)+len(managed))
	for _, entry := range env {
		key := entry
		if pos := strings.IndexByte(entry, '='); pos >= 0 {
			key = entry[:pos]
		}
		if _, replaced := managed[key]; replaced {
			continue
		}
		out = append(out, entry)
	}
	for key, value := range managed {
		out = append(out, key+"="+value)
	}
	return out
}

func readWebSocketClientFrame(r *bufio.Reader) (byte, []byte, error) {
	first, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	if first&0x80 == 0 || first&0x70 != 0 {
		return 0, nil, errors.New("fragmented or reserved WebSocket frame rejected")
	}
	opcode := first & 0x0f
	if second&0x80 == 0 {
		return 0, nil, errors.New("unmasked WebSocket client frame rejected")
	}

	length := uint64(second & 0x7f)
	switch length {
	case 126:
		var raw [2]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(raw[:]))
	case 127:
		var raw [8]byte
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(raw[:])
	}
	if length > adminTerminalWebSocketMaxClientFrame {
		return 0, nil, errors.New("WebSocket client frame exceeds limit")
	}
	if opcode >= 0x8 && length > 125 {
		return 0, nil, errors.New("oversized WebSocket control frame")
	}

	var mask [4]byte
	if _, err := io.ReadFull(r, mask[:]); err != nil {
		return 0, nil, err
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return opcode, payload, nil
}

func (w *terminalWebSocketWriter) writeFrame(opcode byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.w.WriteByte(0x80 | (opcode & 0x0f)); err != nil {
		return err
	}
	length := len(payload)
	switch {
	case length < 126:
		if err := w.w.WriteByte(byte(length)); err != nil {
			return err
		}
	case length <= 65535:
		if err := w.w.WriteByte(126); err != nil {
			return err
		}
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], uint16(length))
		if _, err := w.w.Write(raw[:]); err != nil {
			return err
		}
	default:
		if err := w.w.WriteByte(127); err != nil {
			return err
		}
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], uint64(length))
		if _, err := w.w.Write(raw[:]); err != nil {
			return err
		}
	}
	if len(payload) != 0 {
		if _, err := w.w.Write(payload); err != nil {
			return err
		}
	}
	return w.w.Flush()
}

func closeTerminalPTY(master *os.File, cmd *exec.Cmd) {
	if master != nil {
		_ = master.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGHUP)
	}
}
