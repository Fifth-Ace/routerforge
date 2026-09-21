package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	v2ClientHelloCaptureMaxSeconds = 8
	v2ClientHelloCaptureMaxPCAP    = 2 << 20
	v2ClientHelloCaptureMaxItems   = 24
)

var v2CaptureInterfacePattern = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,32}$`)

type v2ClientHelloCaptureRequest struct {
	DeviceIP  string `json:"device_ip"`
	Interface string `json:"interface,omitempty"`
	Seconds   int    `json:"seconds,omitempty"`
}

type v2ClientHelloCaptureStats struct {
	TCPPackets    int `json:"tcp_packets"`
	TCPPayloads   int `json:"tcp_payload_packets"`
	UDP443Packets int `json:"udp_443_packets"`
	TotalPackets  int `json:"total_packets"`
}

type v2ClientHelloCandidate struct {
	SrcIP         string `json:"src_ip"`
	DstIP         string `json:"dst_ip"`
	DstPort       int    `json:"dst_port"`
	SNI           string `json:"sni,omitempty"`
	Size          int    `json:"size"`
	Valid         bool   `json:"valid"`
	Detail        string `json:"detail"`
	SHA256        string `json:"sha256"`
	ContentBase64 string `json:"content_base64"`
}

func v2ClientHelloCaptureInterface(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw != "" {
		if !v2CaptureInterfacePattern.MatchString(raw) || filepath.Base(raw) != raw {
			return "", errors.New("invalid capture interface")
		}
		if _, err := os.Stat(filepath.Join("/sys/class/net", raw)); err != nil {
			return "", errors.New("capture interface not found")
		}
		return raw, nil
	}
	return "any", nil
}

func v2CaptureClientHellos(ctx context.Context, deviceIP, iface string, seconds int) ([]v2ClientHelloCandidate, string, v2ClientHelloCaptureStats, error) {
	ip := net.ParseIP(strings.TrimSpace(deviceIP))
	if ip == nil {
		return nil, "", v2ClientHelloCaptureStats{}, errors.New("device_ip must be an IPv4 or IPv6 address")
	}
	if seconds <= 0 {
		seconds = 5
	}
	if seconds > v2ClientHelloCaptureMaxSeconds {
		return nil, "", v2ClientHelloCaptureStats{}, errors.New("seconds exceeds capture limit")
	}
	iface, err := v2ClientHelloCaptureInterface(iface)
	if err != nil {
		return nil, "", v2ClientHelloCaptureStats{}, err
	}
	tcpdump, err := exec.LookPath("tcpdump")
	if err != nil {
		if executableFile("/opt/sbin/tcpdump") {
			tcpdump = "/opt/sbin/tcpdump"
		} else if executableFile("/opt/bin/tcpdump") {
			tcpdump = "/opt/bin/tcpdump"
		} else {
			return nil, iface, v2ClientHelloCaptureStats{}, errors.New("tcpdump is not installed")
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	cmd, err := safety.CommandContext(runCtx, tcpdump,
		"-i", iface, "-nn", "-U", "-s", "0", "-c", "256", "-w", "-",
		"src", "host", ip.String(), "and", "(", "tcp", "or", "udp", "dst", "port", "443", ")")
	if err != nil {
		return nil, iface, v2ClientHelloCaptureStats{}, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if stdout.Len() > v2ClientHelloCaptureMaxPCAP {
		return nil, iface, v2ClientHelloCaptureStats{}, errors.New("capture exceeded safety limit")
	}
	if stdout.Len() < 24 {
		if runErr != nil && runCtx.Err() == nil {
			return nil, iface, v2ClientHelloCaptureStats{}, errors.New(strings.TrimSpace(stderr.String()))
		}
		return []v2ClientHelloCandidate{}, iface, v2ClientHelloCaptureStats{}, nil
	}
	stats, err := v2CapturePCAPStats(stdout.Bytes())
	if err != nil {
		return nil, iface, v2ClientHelloCaptureStats{}, err
	}
	items, err := v2ParsePCAPClientHellos(stdout.Bytes())
	if err != nil {
		return nil, iface, stats, err
	}
	return items, iface, stats, nil
}

func v2ParsePCAPClientHellosLegacy(data []byte) ([]v2ClientHelloCandidate, error) {
	if len(data) < 24 {
		return nil, errors.New("pcap is too short")
	}
	var order binary.ByteOrder
	switch {
	case data[0] == 0xd4 && data[1] == 0xc3 && data[2] == 0xb2 && data[3] == 0xa1:
		order = binary.LittleEndian
	case data[0] == 0xa1 && data[1] == 0xb2 && data[2] == 0xc3 && data[3] == 0xd4:
		order = binary.BigEndian
	default:
		return nil, errors.New("unsupported pcap header")
	}
	linkType := order.Uint32(data[20:24])
	offset := 24

	type flow struct {
		src, dst string
		dport    int
		baseSeq  uint32
		buf      []byte
		need     int
	}
	flows := map[string]*flow{}
	seen := map[string]bool{}
	out := []v2ClientHelloCandidate{}

	for offset+16 <= len(data) {
		incl := int(order.Uint32(data[offset+8 : offset+12]))
		offset += 16
		if incl <= 0 || offset+incl > len(data) {
			break
		}
		packet := data[offset : offset+incl]
		offset += incl

		src, dst, proto, l4, ok := v2CaptureL3(linkType, packet)
		if !ok || proto != 6 {
			continue
		}
		sport, dport, seq, payload, ok := v2CaptureTCP(l4)
		if !ok || len(payload) == 0 || dport != 443 {
			continue
		}
		key := src + "|" + sport + ">" + dst
		f := flows[key]
		if f == nil {
			if len(payload) < 9 || payload[0] != 0x16 || payload[1] != 0x03 || payload[5] != 0x01 {
				continue
			}
			need := 5 + int(payload[3])<<8 + int(payload[4])
			if need < 9 || need > 64<<10 {
				continue
			}
			f = &flow{src: src, dst: dst, dport: dport, baseSeq: seq, buf: append([]byte(nil), payload...), need: need}
			flows[key] = f
		} else {
			delta := int(seq - f.baseSeq)
			if delta >= 0 && delta <= len(f.buf) && delta+len(payload) > len(f.buf) {
				f.buf = append(f.buf[:delta], payload...)
			}
		}
		if len(f.buf) < f.need {
			continue
		}
		delete(flows, key)
		record := append([]byte(nil), f.buf[:f.need]...)
		info := v2ValidateClientHello(record)
		if info.SNI != "" && seen[strings.ToLower(info.SNI)] {
			continue
		}
		if info.SNI != "" {
			seen[strings.ToLower(info.SNI)] = true
		}
		out = append(out, v2ClientHelloCandidate{
			SrcIP: f.src, DstIP: f.dst, DstPort: f.dport,
			SNI: info.SNI, Size: len(record), Valid: info.Valid, Detail: info.Detail,
			SHA256: blobSHA256(record), ContentBase64: base64.StdEncoding.EncodeToString(record),
		})
		if len(out) >= v2ClientHelloCaptureMaxItems {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Size < out[j].Size })
	return out, nil
}

func v2ParsePCAPClientHellos(data []byte) ([]v2ClientHelloCandidate, error) {
	return v2ParsePCAPClientHellosRobust(data)
}

func v2CaptureL3(linkType uint32, packet []byte) (string, string, int, []byte, bool) {
	switch linkType {
	case 1:
		if len(packet) < 14 {
			return "", "", 0, nil, false
		}
		etherType := int(packet[12])<<8 | int(packet[13])
		pos := 14
		for etherType == 0x8100 || etherType == 0x88a8 {
			if len(packet) < pos+4 {
				return "", "", 0, nil, false
			}
			etherType = int(packet[pos+2])<<8 | int(packet[pos+3])
			pos += 4
		}
		switch etherType {
		case 0x0800:
			return v2CaptureIPv4(packet[pos:])
		case 0x86dd:
			return v2CaptureIPv6(packet[pos:])
		default:
			return "", "", 0, nil, false
		}
	case 113:
		if len(packet) < 16 {
			return "", "", 0, nil, false
		}
		switch int(packet[14])<<8 | int(packet[15]) {
		case 0x0800:
			return v2CaptureIPv4(packet[16:])
		case 0x86dd:
			return v2CaptureIPv6(packet[16:])
		default:
			return "", "", 0, nil, false
		}
	case 101:
		if len(packet) < 1 {
			return "", "", 0, nil, false
		}
		switch packet[0] >> 4 {
		case 4:
			return v2CaptureIPv4(packet)
		case 6:
			return v2CaptureIPv6(packet)
		default:
			return "", "", 0, nil, false
		}
	case 276:
		if len(packet) < 20 {
			return "", "", 0, nil, false
		}
		switch int(packet[0])<<8 | int(packet[1]) {
		case 0x0800:
			return v2CaptureIPv4(packet[20:])
		case 0x86dd:
			return v2CaptureIPv6(packet[20:])
		default:
			return "", "", 0, nil, false
		}
	default:
		return "", "", 0, nil, false
	}
}

func v2CapturePCAPStats(data []byte) (v2ClientHelloCaptureStats, error) {
	var stats v2ClientHelloCaptureStats
	if len(data) < 24 {
		return stats, errors.New("pcap is too short")
	}
	var order binary.ByteOrder
	switch {
	case data[0] == 0xd4 && data[1] == 0xc3 && data[2] == 0xb2 && data[3] == 0xa1:
		order = binary.LittleEndian
	case data[0] == 0xa1 && data[1] == 0xb2 && data[2] == 0xc3 && data[3] == 0xd4:
		order = binary.BigEndian
	default:
		return stats, errors.New("unsupported pcap header")
	}
	linkType := order.Uint32(data[20:24])
	offset := 24
	for offset+16 <= len(data) {
		incl := int(order.Uint32(data[offset+8 : offset+12]))
		offset += 16
		if incl <= 0 || offset+incl > len(data) {
			break
		}
		packet := data[offset : offset+incl]
		offset += incl
		stats.TotalPackets++
		_, _, proto, l4, ok := v2CaptureL3(linkType, packet)
		if !ok {
			continue
		}
		switch proto {
		case 6:
			stats.TCPPackets++
			_, _, _, payload, ok := v2CaptureTCP(l4)
			if ok && len(payload) > 0 {
				stats.TCPPayloads++
			}
		case 17:
			if len(l4) >= 8 && int(binary.BigEndian.Uint16(l4[2:4])) == 443 {
				stats.UDP443Packets++
			}
		}
	}
	return stats, nil
}

func v2CaptureIPv4(data []byte) (string, string, int, []byte, bool) {
	if len(data) < 20 || data[0]>>4 != 4 {
		return "", "", 0, nil, false
	}
	ihl := int(data[0]&0x0f) * 4
	if ihl < 20 || len(data) < ihl {
		return "", "", 0, nil, false
	}
	if data[6]&0x3f != 0 || data[7] != 0 {
		return "", "", 0, nil, false
	}
	return net.IP(data[12:16]).String(), net.IP(data[16:20]).String(), int(data[9]), data[ihl:], true
}

func v2CaptureIPv6(data []byte) (string, string, int, []byte, bool) {
	if len(data) < 40 || data[0]>>4 != 6 {
		return "", "", 0, nil, false
	}
	if data[6] != 6 {
		// ClientHello capture deliberately supports the common direct TCP case.
		// Extension-header reassembly is not guessed here; unsupported packets
		// are skipped rather than misparsed.
		return "", "", 0, nil, false
	}
	payloadLen := int(binary.BigEndian.Uint16(data[4:6]))
	if payloadLen > 0 && 40+payloadLen > len(data) {
		return "", "", 0, nil, false
	}
	end := len(data)
	if payloadLen > 0 && 40+payloadLen < end {
		end = 40 + payloadLen
	}
	return net.IP(data[8:24]).String(), net.IP(data[24:40]).String(), 6, data[40:end], true
}

func v2CaptureTCP(data []byte) (string, int, uint32, []byte, bool) {
	if len(data) < 20 {
		return "", 0, 0, nil, false
	}
	offset := int(data[12]>>4) * 4
	if offset < 20 || len(data) < offset {
		return "", 0, 0, nil, false
	}
	sport := int(data[0])<<8 | int(data[1])
	dport := int(data[2])<<8 | int(data[3])
	return fmt.Sprintf("%d", sport), dport, binary.BigEndian.Uint32(data[4:8]), data[offset:], true
}

func handleV2ClientHelloCapture(w http.ResponseWriter, r *http.Request) {
	var req v2ClientHelloCaptureRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid capture request"})
		return
	}
	items, iface, stats, err := v2CaptureClientHellos(r.Context(), req.DeviceIP, req.Interface, req.Seconds)
	if err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "not installed") {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "tcpdump_required": strings.Contains(err.Error(), "not installed")})
		return
	}
	hint := ""
	if len(items) == 0 {
		switch {
		case stats.UDP443Packets > 0 && stats.TCPPayloads == 0:
			hint = "Виден UDP/443, но нет TCP ClientHello: браузер, вероятно, использует HTTP/3/QUIC."
		case stats.TCPPackets > 0:
			hint = "TCP-трафик устройства виден, но TLS ClientHello не найден. Открой новое HTTPS-соединение и повтори."
		case stats.TotalPackets == 0:
			hint = "Трафик выбранного устройства не попал в захват. Проверь IP устройства."
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "interface": iface, "device_ip": strings.TrimSpace(req.DeviceIP),
		"count": len(items), "candidates": items, "capture_stats": stats, "hint": hint,
		"read_only_capture": true, "production_mutation": false,
	})
}
