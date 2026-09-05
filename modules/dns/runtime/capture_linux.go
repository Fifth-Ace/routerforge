package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

const (
	ethPAll        = 0x0003
	packetOutgoing = 4
)

func htons(v uint16) uint16 { return (v<<8)&0xff00 | v>>8 }

func captureLoop(store *Store, log *EventLogger) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("packet capture requires root")
	}
	iface, err := net.InterfaceByName("lo")
	if err != nil {
		return fmt.Errorf("interface lo: %w", err)
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(ethPAll)))
	if err != nil {
		return fmt.Errorf("AF_PACKET socket: %w", err)
	}
	defer syscall.Close(fd)
	if err := syscall.Bind(fd, &syscall.SockaddrLinklayer{Protocol: htons(ethPAll), Ifindex: iface.Index}); err != nil {
		return fmt.Errorf("bind lo: %w", err)
	}
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
		if ll != nil && ll.Pkttype != packetOutgoing {
			continue
		}
		handlePacket(store, log, time.Now(), buf[:n])
	}
}

func handlePacket(store *Store, log *EventLogger, now time.Time, pkt []byte) {
	if len(pkt) < 14 {
		return
	}
	switch binary.BigEndian.Uint16(pkt[12:14]) {
	case 0x0800:
		handleIPv4Packet(store, log, now, pkt[14:])
	case 0x86dd:
		handleIPv6Packet(store, log, now, pkt[14:])
	}
}
func handleIPv4Packet(store *Store, log *EventLogger, now time.Time, ip []byte) {
	if len(ip) < 20 || ip[0]>>4 != 4 {
		return
	}
	ihl := int(ip[0]&0x0f) * 4
	if ihl < 20 || len(ip) < ihl {
		return
	}
	if ip[9] == 17 {
		handleUDPDatagram(store, log, now, ip[ihl:])
	}
}
func handleIPv6Packet(store *Store, log *EventLogger, now time.Time, ip []byte) {
	if len(ip) < 40 || ip[0]>>4 != 6 {
		return
	}
	if ip[6] == 17 {
		handleUDPDatagram(store, log, now, ip[40:])
	}
}
func handleUDPDatagram(store *Store, log *EventLogger, now time.Time, udp []byte) {
	if len(udp) < 8 {
		return
	}
	src := binary.BigEndian.Uint16(udp[0:2])
	dst := binary.BigEndian.Uint16(udp[2:4])
	payload := udp[8:]
	d, ok := parseDNSMessage(payload)
	if !ok {
		return
	}
	if _, exists := store.MetaForPort(dst); exists && !d.QR {
		store.RecordQuery(now, "UDP", dst, src, d.ID, d)
		return
	}
	if _, exists := store.MetaForPort(src); exists && d.QR {
		store.RecordResponse(now, src, dst, d.ID, d, log)
		return
	}
	// Catch newly-created proxy ports between discovery refreshes.
	if dst >= 40500 && dst <= 40999 && !d.QR {
		store.RecordQuery(now, "UDP", dst, src, d.ID, d)
	}
}
