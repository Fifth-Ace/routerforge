//go:build linux

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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
	"time"
	"unsafe"
)

const (
	adminTerminalSessionLimit       = 4
	adminTerminalSessionIdleTimeout = 20 * time.Minute
	adminTerminalSessionBufferLimit = 256 << 10
	adminTerminalSessionReadLimit   = 64 << 10
	adminTerminalInputLimit         = 4096
)

type terminalWindowSize struct {
	Rows   uint16
	Cols   uint16
	XPixel uint16
	YPixel uint16
}

type adminTerminalSession struct {
	mu         sync.Mutex
	id         string
	clientID   string
	master     *os.File
	cmd        *exec.Cmd
	output     []byte
	baseCursor int64
	endCursor  int64
	closed     bool
	exitCode   int
	touchedAt  time.Time
}

type adminTerminalSessionManager struct {
	mu       sync.Mutex
	sessions map[string]*adminTerminalSession
	once     sync.Once
}

var terminalSessions = &adminTerminalSessionManager{
	sessions: make(map[string]*adminTerminalSession),
}

type adminTerminalSessionCreateRequest struct {
	Cwd      string `json:"cwd"`
	Cols     int    `json:"cols"`
	Rows     int    `json:"rows"`
	ClientID string `json:"client_id"`
	Confirm  string `json:"confirm"`
}

type adminTerminalSessionInputRequest struct {
	Data string `json:"data"`
}

type adminTerminalSessionResizeRequest struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

func registerAdminTerminalSessionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/terminal/session", mutationOnly(handleAdminTerminalSessionCreate))
	mux.HandleFunc("/v1/terminal/session/", adminTerminalSessionOnly(handleAdminTerminalSessionRequest))
}

func adminTerminalSessionOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(adminMutationAuthorizationHeader) != adminMutationAuthorizationValue {
			writeJSON(w, http.StatusForbidden, map[string]any{
				"error": "authorized RouterForge Core session required",
			})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func handleAdminTerminalSessionCreate(w http.ResponseWriter, r *http.Request) {
	var request adminTerminalSessionCreateRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid terminal session request"})
		return
	}
	if request.Confirm != "CONNECT" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "confirm must equal CONNECT"})
		return
	}

	cwd := strings.TrimSpace(request.Cwd)
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

	clientID := strings.ToLower(strings.TrimSpace(request.ClientID))
	if !terminalClientIDValid(clientID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid terminal client_id"})
		return
	}

	cols, rows := normalizeTerminalSize(request.Cols, request.Rows)
	session, err := terminalSessions.create(resolved.Canonical, cols, rows, clientID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, errTerminalSessionLimit) {
			status = http.StatusTooManyRequests
		}
		writeJSON(w, status, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"ok":         true,
		"session_id": session.id,
		"cwd":        resolved.Lexical,
		"cols":       cols,
		"rows":       rows,
		"shell":      "/bin/sh",
		"mode":       "entware-pty",
	})
}

func handleAdminTerminalSessionRequest(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseTerminalSessionPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	session := terminalSessions.get(id)
	if session == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "terminal session not found"})
		return
	}

	switch action {
	case "output":
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "GET required"})
			return
		}
		handleAdminTerminalSessionOutput(w, r, session)
	case "input":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
			return
		}
		handleAdminTerminalSessionInput(w, r, session)
	case "resize":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
			return
		}
		handleAdminTerminalSessionResize(w, r, session)
	case "close":
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "POST required"})
			return
		}
		terminalSessions.remove(id)
		session.close()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "session_id": id})
	default:
		http.NotFound(w, r)
	}
}

func handleAdminTerminalSessionOutput(w http.ResponseWriter, r *http.Request, session *adminTerminalSession) {
	cursor := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("cursor")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid terminal cursor"})
			return
		}
		cursor = value
	}

	data, next, dropped, closed, exitCode := session.readOutput(cursor)
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id":  session.id,
		"data_base64": base64.StdEncoding.EncodeToString(data),
		"cursor":      next,
		"dropped":     dropped,
		"closed":      closed,
		"exit_code":   exitCode,
	})
}

func handleAdminTerminalSessionInput(w http.ResponseWriter, r *http.Request, session *adminTerminalSession) {
	var request adminTerminalSessionInputRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid terminal input request"})
		return
	}
	if request.Data == "" {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "written": 0})
		return
	}
	if len(request.Data) > adminTerminalInputLimit {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":           "terminal input exceeds limit",
			"max_input_bytes": adminTerminalInputLimit,
		})
		return
	}
	written, err := session.writeInput([]byte(request.Data))
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "terminal session is closed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "written": written})
}

func handleAdminTerminalSessionResize(w http.ResponseWriter, r *http.Request, session *adminTerminalSession) {
	var request adminTerminalSessionResizeRequest
	if err := decodeMutationJSON(w, r, &request); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid terminal resize request"})
		return
	}
	cols, rows := normalizeTerminalSize(request.Cols, request.Rows)
	if err := session.resize(cols, rows); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "terminal session is closed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "cols": cols, "rows": rows})
}

func parseTerminalSessionPath(path string) (string, string, bool) {
	const prefix = "/v1/terminal/session/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	for _, ch := range parts[0] {
		if !((ch >= 'a' && ch <= 'f') || (ch >= '0' && ch <= '9')) {
			return "", "", false
		}
	}
	if len(parts[0]) != 32 {
		return "", "", false
	}
	switch parts[1] {
	case "output", "input", "resize", "close":
		return parts[0], parts[1], true
	default:
		return "", "", false
	}
}

func normalizeTerminalSize(cols, rows int) (int, int) {
	if cols < 20 || cols > 400 {
		cols = 80
	}
	if rows < 5 || rows > 200 {
		rows = 24
	}
	return cols, rows
}

func terminalClientIDValid(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, ch := range value {
		if !((ch >= 'a' && ch <= 'f') || (ch >= '0' && ch <= '9')) {
			return false
		}
	}
	return true
}

var errTerminalSessionLimit = errors.New("terminal session limit reached")

func (m *adminTerminalSessionManager) create(cwd string, cols, rows int, clientID string) (*adminTerminalSession, error) {
	m.once.Do(func() { go m.janitor() })
	m.cleanupInactive()

	var replaced []*adminTerminalSession
	m.mu.Lock()
	for id, existing := range m.sessions {
		if existing.clientID == clientID {
			delete(m.sessions, id)
			replaced = append(replaced, existing)
		}
	}
	if len(m.sessions) >= adminTerminalSessionLimit {
		m.mu.Unlock()
		for _, existing := range replaced {
			existing.close()
		}
		return nil, errTerminalSessionLimit
	}
	m.mu.Unlock()
	for _, existing := range replaced {
		existing.close()
	}

	id, err := randomTerminalSessionID()
	if err != nil {
		return nil, err
	}

	shellScript := `cd "$1" || exit 1
if [ -r /opt/etc/profile ]; then
	. /opt/etc/profile
fi
export PATH=/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin
export HOME=/opt/root
export USER=root
export LOGNAME=root
export SHELL=/bin/sh
export TERM=xterm-256color
exec /bin/sh -i
`
	cmd := exec.Command("/bin/sh", "-c", shellScript, "routerforge-terminal", cwd)
	cmd.Env = terminalEnvironment(os.Environ())
	master, err := startTerminalPTY(cmd, cols, rows)
	if err != nil {
		return nil, fmt.Errorf("cannot start Entware PTY: %w", err)
	}

	session := &adminTerminalSession{
		id:        id,
		clientID:  clientID,
		master:    master,
		cmd:       cmd,
		touchedAt: time.Now(),
	}

	var superseded []*adminTerminalSession
	m.mu.Lock()
	for existingID, existing := range m.sessions {
		if existing.clientID == clientID {
			delete(m.sessions, existingID)
			superseded = append(superseded, existing)
		}
	}
	if len(m.sessions) >= adminTerminalSessionLimit {
		m.mu.Unlock()
		for _, existing := range superseded {
			existing.close()
		}
		session.close()
		return nil, errTerminalSessionLimit
	}
	m.sessions[id] = session
	m.mu.Unlock()
	for _, existing := range superseded {
		existing.close()
	}

	go session.capture()
	go session.wait()
	return session, nil
}

func (m *adminTerminalSessionManager) get(id string) *adminTerminalSession {
	m.mu.Lock()
	session := m.sessions[id]
	m.mu.Unlock()
	if session != nil {
		session.touch()
	}
	return session
}

func (m *adminTerminalSessionManager) remove(id string) {
	m.mu.Lock()
	delete(m.sessions, id)
	m.mu.Unlock()
}

func (m *adminTerminalSessionManager) cleanupInactive() {
	now := time.Now()
	var stale []*adminTerminalSession
	m.mu.Lock()
	for id, session := range m.sessions {
		if session.closedState() || session.idleFor(now) > adminTerminalSessionIdleTimeout {
			delete(m.sessions, id)
			stale = append(stale, session)
		}
	}
	m.mu.Unlock()
	for _, session := range stale {
		session.close()
	}
}

func (m *adminTerminalSessionManager) janitor() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.cleanupInactive()
	}
}

func randomTerminalSessionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func terminalEnvironment(env []string) []string {
	replace := map[string]string{
		"PATH":    "/opt/sbin:/opt/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME":    "/opt/root",
		"USER":    "root",
		"LOGNAME": "root",
		"SHELL":   "/bin/sh",
		"TERM":    "xterm-256color",
		"LANG":    "C.UTF-8",
	}
	out := make([]string, 0, len(env)+len(replace))
	for _, entry := range env {
		key := entry
		if pos := strings.IndexByte(entry, '='); pos >= 0 {
			key = entry[:pos]
		}
		if _, managed := replace[key]; managed {
			continue
		}
		out = append(out, entry)
	}
	for key, value := range replace {
		out = append(out, key+"="+value)
	}
	return out
}

func startTerminalPTY(cmd *exec.Cmd, cols, rows int) (*os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, err
	}

	unlock := int32(0)
	if err := terminalIoctl(master.Fd(), uintptr(syscall.TIOCSPTLCK), unsafe.Pointer(&unlock)); err != nil {
		_ = master.Close()
		return nil, err
	}

	var ptyNumber uint32
	if err := terminalIoctl(master.Fd(), uintptr(syscall.TIOCGPTN), unsafe.Pointer(&ptyNumber)); err != nil {
		_ = master.Close()
		return nil, err
	}

	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", ptyNumber), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		_ = master.Close()
		return nil, err
	}

	if err := setTerminalWindowSize(master.Fd(), cols, rows); err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, err
	}

	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}

	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = master.Close()
		return nil, err
	}
	_ = slave.Close()
	return master, nil
}

func terminalIoctl(fd uintptr, request uintptr, value unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(value))
	if errno != 0 {
		return errno
	}
	return nil
}

func setTerminalWindowSize(fd uintptr, cols, rows int) error {
	size := terminalWindowSize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}
	return terminalIoctl(fd, uintptr(syscall.TIOCSWINSZ), unsafe.Pointer(&size))
}

func (s *adminTerminalSession) touch() {
	s.mu.Lock()
	s.touchedAt = time.Now()
	s.mu.Unlock()
}

func (s *adminTerminalSession) idleFor(now time.Time) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return now.Sub(s.touchedAt)
}

func (s *adminTerminalSession) closedState() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func (s *adminTerminalSession) capture() {
	buffer := make([]byte, 8192)
	for {
		n, err := s.master.Read(buffer)
		if n > 0 {
			s.appendOutput(buffer[:n])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, syscall.EIO) {
				s.appendOutput([]byte("\r\n[terminal transport closed]\r\n"))
			}
			return
		}
	}
}

func (s *adminTerminalSession) wait() {
	err := s.cmd.Wait()
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	s.mu.Lock()
	s.closed = true
	s.exitCode = exitCode
	s.touchedAt = time.Now()
	s.mu.Unlock()
}

func (s *adminTerminalSession) appendOutput(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.output = append(s.output, data...)
	s.endCursor += int64(len(data))
	if len(s.output) > adminTerminalSessionBufferLimit {
		drop := len(s.output) - adminTerminalSessionBufferLimit
		copy(s.output, s.output[drop:])
		s.output = s.output[:len(s.output)-drop]
		s.baseCursor += int64(drop)
	}
	s.touchedAt = time.Now()
}

func (s *adminTerminalSession) readOutput(cursor int64) ([]byte, int64, bool, bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dropped := false
	if cursor < s.baseCursor {
		cursor = s.baseCursor
		dropped = true
	}
	if cursor > s.endCursor {
		cursor = s.endCursor
	}
	start := int(cursor - s.baseCursor)
	available := len(s.output) - start
	if available < 0 {
		available = 0
	}
	if available > adminTerminalSessionReadLimit {
		available = adminTerminalSessionReadLimit
	}
	data := append([]byte(nil), s.output[start:start+available]...)
	next := cursor + int64(available)
	s.touchedAt = time.Now()
	return data, next, dropped, s.closed, s.exitCode
}

func (s *adminTerminalSession) writeInput(data []byte) (int, error) {
	s.mu.Lock()
	if s.closed || s.master == nil {
		s.mu.Unlock()
		return 0, errors.New("terminal session closed")
	}
	master := s.master
	s.touchedAt = time.Now()
	s.mu.Unlock()
	return master.Write(data)
}

func (s *adminTerminalSession) resize(cols, rows int) error {
	s.mu.Lock()
	if s.closed || s.master == nil {
		s.mu.Unlock()
		return errors.New("terminal session closed")
	}
	fd := s.master.Fd()
	s.touchedAt = time.Now()
	s.mu.Unlock()
	return setTerminalWindowSize(fd, cols, rows)
}

func (s *adminTerminalSession) close() {
	s.mu.Lock()
	if s.closed && s.master == nil {
		s.mu.Unlock()
		return
	}
	master := s.master
	s.master = nil
	cmd := s.cmd
	s.closed = true
	s.touchedAt = time.Now()
	s.mu.Unlock()

	if master != nil {
		_ = master.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGHUP)
	}
}
