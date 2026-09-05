package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

type DNSMessage struct {
	ID      uint16
	QR      bool
	RCode   uint8
	QName   string
	QType   uint16
	QDCount uint16
}

func parseDNSMessage(msg []byte) (DNSMessage, bool) {
	var d DNSMessage
	if len(msg) < 12 {
		return d, false
	}
	d.ID = binary.BigEndian.Uint16(msg[0:2])
	flags := binary.BigEndian.Uint16(msg[2:4])
	d.QR = flags&0x8000 != 0
	d.RCode = uint8(flags & 0x000f)
	d.QDCount = binary.BigEndian.Uint16(msg[4:6])
	if d.QDCount == 0 {
		return d, true
	}
	name, off, ok := parseDNSName(msg, 12)
	if !ok || off+4 > len(msg) {
		return DNSMessage{}, false
	}
	d.QName = strings.TrimSuffix(strings.ToLower(name), ".")
	d.QType = binary.BigEndian.Uint16(msg[off : off+2])
	return d, true
}

func parseDNSName(msg []byte, off int) (string, int, bool) {
	labels := make([]string, 0, 8)
	jumped := false
	nextOff := -1
	seen := 0
	for {
		if off >= len(msg) || seen > 128 {
			return "", 0, false
		}
		seen++
		l := int(msg[off])
		if l == 0 {
			off++
			if !jumped {
				nextOff = off
			}
			break
		}
		if l&0xc0 == 0xc0 {
			if off+1 >= len(msg) {
				return "", 0, false
			}
			ptr := int(binary.BigEndian.Uint16(msg[off:off+2]) & 0x3fff)
			if ptr >= len(msg) {
				return "", 0, false
			}
			if !jumped {
				nextOff = off + 2
				jumped = true
			}
			off = ptr
			continue
		}
		if l > 63 || off+1+l > len(msg) {
			return "", 0, false
		}
		labels = append(labels, string(msg[off+1:off+1+l]))
		off += 1 + l
	}
	if nextOff < 0 {
		nextOff = off
	}
	return strings.Join(labels, "."), nextOff, true
}

func qtypeName(q uint16) string {
	switch q {
	case 1:
		return "A"
	case 2:
		return "NS"
	case 5:
		return "CNAME"
	case 12:
		return "PTR"
	case 15:
		return "MX"
	case 16:
		return "TXT"
	case 28:
		return "AAAA"
	case 33:
		return "SRV"
	case 64:
		return "SVCB"
	case 65:
		return "HTTPS"
	default:
		return fmt.Sprintf("TYPE%d", q)
	}
}

func rcodeName(r uint8) string {
	switch r {
	case 0:
		return "NOERROR"
	case 1:
		return "FORMERR"
	case 2:
		return "SERVFAIL"
	case 3:
		return "NXDOMAIN"
	case 4:
		return "NOTIMP"
	case 5:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", r)
	}
}

func buildDNSQuery(id uint16, name string, qtype uint16) []byte {
	name = strings.TrimSuffix(name, ".")
	b := make([]byte, 12, 512)
	binary.BigEndian.PutUint16(b[0:2], id)
	binary.BigEndian.PutUint16(b[2:4], 0x0100) // recursion desired
	binary.BigEndian.PutUint16(b[4:6], 1)
	for _, label := range strings.Split(name, ".") {
		if label == "" || len(label) > 63 {
			continue
		}
		b = append(b, byte(len(label)))
		b = append(b, []byte(label)...)
	}
	b = append(b, 0, 0, 1, 0, 1)
	if qtype != 1 {
		binary.BigEndian.PutUint16(b[len(b)-4:len(b)-2], qtype)
	}
	return b
}
