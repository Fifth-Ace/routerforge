package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
)

func v2TestPCAPWithClientHello(t *testing.T, hello []byte) []byte {
	t.Helper()
	eth := make([]byte, 14)
	eth[12], eth[13] = 0x08, 0x00
	ip := make([]byte, 20)
	ip[0] = 0x45
	ip[9] = 6
	copy(ip[12:16], net.ParseIP("192.168.1.50").To4())
	copy(ip[16:20], net.ParseIP("93.184.216.34").To4())
	tcp := make([]byte, 20)
	binary.BigEndian.PutUint16(tcp[0:2], 50000)
	binary.BigEndian.PutUint16(tcp[2:4], 443)
	binary.BigEndian.PutUint32(tcp[4:8], 100)
	tcp[12] = 5 << 4
	packet := append(append(append([]byte{}, eth...), ip...), tcp...)
	packet = append(packet, hello...)

	var out bytes.Buffer
	out.Write([]byte{0xd4, 0xc3, 0xb2, 0xa1})
	_ = binary.Write(&out, binary.LittleEndian, uint16(2))
	_ = binary.Write(&out, binary.LittleEndian, uint16(4))
	_ = binary.Write(&out, binary.LittleEndian, int32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(65535))
	_ = binary.Write(&out, binary.LittleEndian, uint32(1))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(0))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(packet)))
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(packet)))
	out.Write(packet)
	return out.Bytes()
}

func TestParsePCAPClientHello(t *testing.T) {
	hello, err := v2GenerateClientHello("example.com")
	if err != nil {
		t.Fatal(err)
	}
	items, err := v2ParsePCAPClientHellos(v2TestPCAPWithClientHello(t, hello))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	if !items[0].Valid || items[0].SNI != "example.com" {
		t.Fatalf("candidate=%+v", items[0])
	}
}

func TestCaptureRejectsInvalidIP(t *testing.T) {
	_, _, err := v2CaptureClientHellos(context.Background(), "not-an-ip", "br0", 1)
	if err == nil {
		t.Fatal("invalid device ip accepted")
	}
}
