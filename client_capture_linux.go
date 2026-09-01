package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 10 || (v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31) || (v4[0] == 192 && v4[1] == 168) || (v4[0] == 169 && v4[1] == 254)
	}
	return ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

func clientCaptureLoop(store *Store, log *EventLogger) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("client packet capture requires root")
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(ethPAll)))
	if err != nil {
		return fmt.Errorf("client AF_PACKET socket: %w", err)
	}
	defer syscall.Close(fd)
	buf := make([]byte, 65535)
	for {
		n, sa, err := syscall.Recvfrom(fd, buf, 0)
		if err != nil {
			if err == syscall.EINTR {
				continue
			}
			return err
		}
		ll, _ := sa.(*syscall.SockaddrLinklayer)
		outgoing := ll != nil && ll.Pkttype == packetOutgoing
		handleClientPacket(store, time.Now(), buf[:n], outgoing)
	}
}

func handleClientPacket(store *Store, now time.Time, pkt []byte, outgoing bool) {
	if len(pkt) < 14 {
		return
	}
	srcMAC := net.HardwareAddr(pkt[6:12]).String()
	dstMAC := net.HardwareAddr(pkt[0:6]).String()
	off := 14
	etherType := binary.BigEndian.Uint16(pkt[12:14])
	for etherType == 0x8100 || etherType == 0x88a8 {
		if len(pkt) < off+4 {
			return
		}
		etherType = binary.BigEndian.Uint16(pkt[off+2 : off+4])
		off += 4
	}
	if len(pkt) <= off {
		return
	}
	switch etherType {
	case 0x0800:
		handleClientIPv4(store, now, srcMAC, dstMAC, pkt[off:], outgoing)
	case 0x86dd:
		handleClientIPv6(store, now, srcMAC, dstMAC, pkt[off:], outgoing)
	}
}

func handleClientIPv4(store *Store, now time.Time, srcMAC, dstMAC string, ip []byte, outgoing bool) {
	if len(ip) < 20 || ip[0]>>4 != 4 {
		return
	}
	ihl := int(ip[0]&0x0f) * 4
	if ihl < 20 || len(ip) < ihl {
		return
	}
	srcIP := net.IP(ip[12:16]).String()
	dstIP := net.IP(ip[16:20]).String()
	switch ip[9] {
	case 17:
		handleClientUDP(store, now, srcIP, dstIP, srcMAC, dstMAC, ip[ihl:], outgoing)
	case 6:
		handleClientTCP(store, now, srcIP, dstIP, srcMAC, dstMAC, ip[ihl:], outgoing)
	}
}

func handleClientIPv6(store *Store, now time.Time, srcMAC, dstMAC string, ip []byte, outgoing bool) {
	if len(ip) < 40 || ip[0]>>4 != 6 {
		return
	}
	srcIP := net.IP(ip[8:24]).String()
	dstIP := net.IP(ip[24:40]).String()
	// Keenetic client DNS is normally plain UDP/TCP 53. Extension-header
	// reassembly is intentionally left out; such packets are uncommon here.
	switch ip[6] {
	case 17:
		handleClientUDP(store, now, srcIP, dstIP, srcMAC, dstMAC, ip[40:], outgoing)
	case 6:
		handleClientTCP(store, now, srcIP, dstIP, srcMAC, dstMAC, ip[40:], outgoing)
	}
}

func handleClientUDP(store *Store, now time.Time, srcIP, dstIP, srcMAC, dstMAC string, udp []byte, outgoing bool) {
	if len(udp) < 8 {
		return
	}
	srcPort := binary.BigEndian.Uint16(udp[0:2])
	dstPort := binary.BigEndian.Uint16(udp[2:4])
	d, ok := parseDNSMessage(udp[8:])
	if !ok {
		return
	}
	if !outgoing && dstPort == 53 && !d.QR && isPrivateOrLocalIP(net.ParseIP(srcIP)) {
		store.RecordClientQuery(now, "UDP", srcIP, srcMAC, d.ID, d)
		return
	}
	if outgoing && srcPort == 53 && d.QR && isPrivateOrLocalIP(net.ParseIP(dstIP)) {
		store.RecordClientResponse(now, "UDP", dstIP, dstMAC, d.ID, d)
	}
}

func handleClientTCP(store *Store, now time.Time, srcIP, dstIP, srcMAC, dstMAC string, tcp []byte, outgoing bool) {
	if len(tcp) < 20 {
		return
	}
	srcPort := binary.BigEndian.Uint16(tcp[0:2])
	dstPort := binary.BigEndian.Uint16(tcp[2:4])
	if (!outgoing && dstPort != 53) || (outgoing && srcPort != 53) {
		return
	}
	off := int((tcp[12] >> 4) * 4)
	if off < 20 || len(tcp) < off+2 {
		return
	}
	payload := tcp[off:]
	n := int(binary.BigEndian.Uint16(payload[:2]))
	if n < 12 || len(payload) < 2+n {
		return // best-effort: segmented DNS/TCP is not reassembled yet
	}
	d, ok := parseDNSMessage(payload[2 : 2+n])
	if !ok {
		return
	}
	if !outgoing && !d.QR && isPrivateOrLocalIP(net.ParseIP(srcIP)) {
		store.RecordClientQuery(now, "TCP", srcIP, srcMAC, d.ID, d)
		return
	}
	if outgoing && d.QR && isPrivateOrLocalIP(net.ParseIP(dstIP)) {
		store.RecordClientResponse(now, "TCP", dstIP, dstMAC, d.ID, d)
	}
}
