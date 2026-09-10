//go:build linux

package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"testing"
)

func TestWebSocketAcceptRFCExample(t *testing.T) {
	got := webSocketAccept("dGhlIHNhbXBsZSBub25jZQ==")
	if got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("accept=%q", got)
	}
}

func TestReadWebSocketClientBinaryFrame(t *testing.T) {
	payload := []byte("echo ok\n")
	mask := [4]byte{1, 2, 3, 4}
	var wire bytes.Buffer
	wire.WriteByte(0x82)
	wire.WriteByte(0x80 | byte(len(payload)))
	wire.Write(mask[:])
	for i, value := range payload {
		wire.WriteByte(value ^ mask[i%4])
	}
	opcode, got, err := readWebSocketClientFrame(bufio.NewReader(&wire))
	if err != nil {
		t.Fatal(err)
	}
	if opcode != 0x2 || !bytes.Equal(got, payload) {
		t.Fatalf("opcode=%d payload=%q", opcode, got)
	}
}

func TestReadWebSocketClientRejectsUnmasked(t *testing.T) {
	wire := bytes.NewBuffer([]byte{0x82, 0x01, 'x'})
	if _, _, err := readWebSocketClientFrame(bufio.NewReader(wire)); err == nil {
		t.Fatal("unmasked frame accepted")
	}
}

func TestReadWebSocketClientRejectsOversized(t *testing.T) {
	var wire bytes.Buffer
	wire.WriteByte(0x82)
	wire.WriteByte(0x80 | 127)
	var raw [8]byte
	binary.BigEndian.PutUint64(raw[:], adminTerminalWebSocketMaxClientFrame+1)
	wire.Write(raw[:])
	if _, _, err := readWebSocketClientFrame(bufio.NewReader(&wire)); err == nil {
		t.Fatal("oversized frame accepted")
	}
}
