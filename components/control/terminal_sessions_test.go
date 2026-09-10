//go:build linux

package main

import (
	"bytes"
	"testing"
)

func TestParseTerminalSessionPath(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef"
	gotID, action, ok := parseTerminalSessionPath("/v1/terminal/session/" + id + "/output")
	if !ok || gotID != id || action != "output" {
		t.Fatalf("id=%q action=%q ok=%v", gotID, action, ok)
	}
	if _, _, ok := parseTerminalSessionPath("/v1/terminal/session/not-safe/output"); ok {
		t.Fatal("unsafe session id accepted")
	}
}

func TestTerminalOutputBufferIsBounded(t *testing.T) {
	session := &adminTerminalSession{}
	payload := bytes.Repeat([]byte("x"), adminTerminalSessionBufferLimit+4096)
	session.appendOutput(payload)

	if len(session.output) != adminTerminalSessionBufferLimit {
		t.Fatalf("buffer len=%d want=%d", len(session.output), adminTerminalSessionBufferLimit)
	}
	if session.baseCursor != 4096 {
		t.Fatalf("base cursor=%d want=4096", session.baseCursor)
	}

	data, cursor, dropped, _, _ := session.readOutput(0)
	if !dropped {
		t.Fatal("expected dropped=true for stale cursor")
	}
	if len(data) != adminTerminalSessionReadLimit {
		t.Fatalf("read len=%d want=%d", len(data), adminTerminalSessionReadLimit)
	}
	if cursor != 4096+adminTerminalSessionReadLimit {
		t.Fatalf("cursor=%d", cursor)
	}
}

func TestNormalizeTerminalSize(t *testing.T) {
	cols, rows := normalizeTerminalSize(0, 0)
	if cols != 80 || rows != 24 {
		t.Fatalf("default size=%dx%d", cols, rows)
	}
	cols, rows = normalizeTerminalSize(132, 48)
	if cols != 132 || rows != 48 {
		t.Fatalf("valid size=%dx%d", cols, rows)
	}
}
