package main

import (
	"encoding/binary"
	"testing"
)

func runClassicBPF(t *testing.T, program []classicBPFInstruction, packet []byte) uint32 {
	t.Helper()
	var a uint32
	var x uint32

	for pc := 0; pc < len(program); {
		instruction := program[pc]
		switch instruction.Code {
		case bpfClassLD | bpfSizeH | bpfModeABS:
			offset := int(instruction.K)
			if offset < 0 || offset+2 > len(packet) {
				return 0
			}
			a = uint32(binary.BigEndian.Uint16(packet[offset : offset+2]))
			pc++
		case bpfClassLD | bpfSizeB | bpfModeABS:
			offset := int(instruction.K)
			if offset < 0 || offset >= len(packet) {
				return 0
			}
			a = uint32(packet[offset])
			pc++
		case bpfClassLDX | bpfSizeB | bpfModeMSH:
			offset := int(instruction.K)
			if offset < 0 || offset >= len(packet) {
				return 0
			}
			x = uint32(packet[offset]&0x0f) << 2
			pc++
		case bpfClassLD | bpfSizeH | bpfModeIND:
			offset := int(x + instruction.K)
			if offset < 0 || offset+2 > len(packet) {
				return 0
			}
			a = uint32(binary.BigEndian.Uint16(packet[offset : offset+2]))
			pc++
		case bpfClassJMP | bpfOpJEQ | bpfSrcK:
			if a == instruction.K {
				pc += int(instruction.Jt) + 1
			} else {
				pc += int(instruction.Jf) + 1
			}
		case bpfClassJMP | bpfOpJA:
			pc += int(instruction.K) + 1
		case bpfClassRET | bpfSrcK:
			return instruction.K
		default:
			t.Fatalf("unsupported classic BPF instruction code %#x at pc=%d", instruction.Code, pc)
		}
	}

	return 0
}

func ethernetFrame(etherType uint16, payload []byte, tags ...uint16) []byte {
	packet := make([]byte, 12)
	for _, tag := range tags {
		var field [2]byte
		binary.BigEndian.PutUint16(field[:], tag)
		packet = append(packet, field[:]...)
		packet = append(packet, 0, 0)
	}
	var field [2]byte
	binary.BigEndian.PutUint16(field[:], etherType)
	packet = append(packet, field[:]...)
	packet = append(packet, payload...)
	return packet
}

func ipv4Transport(protocol byte, srcPort, dstPort uint16) []byte {
	packet := make([]byte, 20+20)
	packet[0] = 0x45
	packet[9] = protocol
	binary.BigEndian.PutUint16(packet[20:22], srcPort)
	binary.BigEndian.PutUint16(packet[22:24], dstPort)
	return packet
}

func ipv6Transport(nextHeader byte, srcPort, dstPort uint16) []byte {
	packet := make([]byte, 40+20)
	packet[0] = 0x60
	packet[6] = nextHeader
	binary.BigEndian.PutUint16(packet[40:42], srcPort)
	binary.BigEndian.PutUint16(packet[42:44], dstPort)
	return packet
}

func TestProxyCaptureFilterProgram(t *testing.T) {
	tests := []struct {
		name   string
		packet []byte
		want   bool
	}{
		{"ipv4_udp", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoUDP, 50000, 40500)), true},
		{"ipv6_udp", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoUDP, 40500, 50000)), true},
		{"ipv4_tcp", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoTCP, 50000, 40500)), false},
		{"ipv6_tcp", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoTCP, 40500, 50000)), false},
		{"arp", ethernetFrame(0x0806, make([]byte, 32)), false},
		{"short", []byte{1, 2, 3}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runClassicBPF(t, proxyCaptureFilterProgram, test.packet) != 0
			if got != test.want {
				t.Fatalf("proxy filter result=%v want=%v", got, test.want)
			}
		})
	}
}

func TestClientCaptureFilterProgram(t *testing.T) {
	tests := []struct {
		name   string
		packet []byte
		want   bool
	}{
		{"ipv4_udp_src53", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoUDP, 53, 42000)), true},
		{"ipv4_udp_dst53", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoUDP, 42000, 53)), true},
		{"ipv4_tcp_dst53", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoTCP, 42000, 53)), true},
		{"ipv6_udp_src53", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoUDP, 53, 42000)), true},
		{"ipv6_tcp_dst53", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoTCP, 42000, 53)), true},
		{"ipv4_udp_non_dns", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoUDP, 42000, 42001)), false},
		{"ipv6_tcp_non_dns", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoTCP, 42000, 42001)), false},
		{"ipv6_extension_header", ethernetFrame(etherTypeIPv6, ipv6Transport(0, 42000, 53)), false},
		{"single_vlan_ipv4_dns", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoUDP, 42000, 53), etherTypeVLAN), true},
		{"single_qinq_ipv6_dns", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoTCP, 53, 42000), etherTypeQinQ), true},
		{"double_vlan_ipv6_dns", ethernetFrame(etherTypeIPv6, ipv6Transport(ipProtoUDP, 42000, 53), etherTypeVLAN, etherTypeQinQ), true},
		{"double_vlan_non_dns", ethernetFrame(etherTypeIPv4, ipv4Transport(ipProtoUDP, 42000, 42001), etherTypeVLAN, etherTypeVLAN), false},
		{"triple_vlan_fallback", ethernetFrame(0x0806, make([]byte, 32), etherTypeVLAN, etherTypeQinQ, etherTypeVLAN), true},
		{"arp", ethernetFrame(0x0806, make([]byte, 32)), false},
		{"single_vlan_arp", ethernetFrame(0x0806, make([]byte, 32), etherTypeVLAN), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runClassicBPF(t, clientCaptureFilterProgram, test.packet) != 0
			if got != test.want {
				t.Fatalf("client filter result=%v want=%v", got, test.want)
			}
		})
	}
}
