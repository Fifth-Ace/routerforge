package main

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type v2ClientHelloForgeRequest struct {
	ContentBase64 string `json:"content_base64"`
	SNI           string `json:"sni"`
}

type v2ClientHelloForgeInfo struct {
	Valid             bool     `json:"valid"`
	SNI               string   `json:"sni,omitempty"`
	Size              int      `json:"size"`
	Detail            string   `json:"detail"`
	ALPN              []string `json:"alpn"`
	SupportedVersions []string `json:"supported_versions"`
	ExtensionCount    int      `json:"extension_count"`
}

type v2ClientHelloLayout struct {
	bodyStart       int
	extLenPosBody   int
	extStartBody    int
	extEndBody      int
	sniStartBody    int
	sniEndBody      int
	hasSNIExtension bool
}

type v2ClientHelloCapability struct {
	DynamicCloneAvailable bool   `json:"dynamic_clone_available"`
	DynamicCloneSource    string `json:"dynamic_clone_source,omitempty"`
	StaticForgeAvailable  bool   `json:"static_forge_available"`
	CaptureAvailable      bool   `json:"capture_available"`
	TCPDumpAvailable      bool   `json:"tcpdump_available"`
}

func registerClientHelloForgeV2Routes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/clienthello/forge", mutationOnly(handleV2ClientHelloForge))
	mux.HandleFunc("/v1/clienthello/capabilities", getOnly(handleV2ClientHelloCapabilities))
}

func v2ParseClientHelloLayout(data []byte) (v2ClientHelloLayout, error) {
	var layout v2ClientHelloLayout
	if len(data) < 9 || data[0] != 0x16 || data[1] != 0x03 || data[5] != 0x01 {
		return layout, errors.New("not a TLS ClientHello record")
	}
	recordLen := int(binary.BigEndian.Uint16(data[3:5]))
	if 5+recordLen != len(data) {
		return layout, errors.New("TLS record length mismatch")
	}
	handshakeLen := int(data[6])<<16 | int(data[7])<<8 | int(data[8])
	if 9+handshakeLen != len(data) {
		return layout, errors.New("ClientHello handshake length mismatch")
	}
	body := data[9:]
	if len(body) < 34 {
		return layout, errors.New("ClientHello body is too short")
	}
	p := 34
	if p >= len(body) {
		return layout, errors.New("ClientHello session id is missing")
	}
	sessionLen := int(body[p])
	p++
	if p+sessionLen > len(body) {
		return layout, errors.New("ClientHello session id is truncated")
	}
	p += sessionLen
	if p+2 > len(body) {
		return layout, errors.New("ClientHello cipher list is missing")
	}
	cipherLen := int(binary.BigEndian.Uint16(body[p : p+2]))
	p += 2
	if cipherLen == 0 || cipherLen%2 != 0 || p+cipherLen > len(body) {
		return layout, errors.New("ClientHello cipher list is invalid")
	}
	p += cipherLen
	if p >= len(body) {
		return layout, errors.New("ClientHello compression methods are missing")
	}
	compLen := int(body[p])
	p++
	if p+compLen > len(body) {
		return layout, errors.New("ClientHello compression methods are truncated")
	}
	p += compLen
	if p == len(body) {
		return layout, errors.New("ClientHello has no extensions")
	}
	if p+2 > len(body) {
		return layout, errors.New("ClientHello extensions length is missing")
	}
	layout.bodyStart = 9
	layout.extLenPosBody = p
	extLen := int(binary.BigEndian.Uint16(body[p : p+2]))
	p += 2
	layout.extStartBody = p
	layout.extEndBody = p + extLen
	if layout.extEndBody != len(body) {
		return layout, errors.New("ClientHello extensions length mismatch")
	}
	for p+4 <= layout.extEndBody {
		start := p
		kind := binary.BigEndian.Uint16(body[p : p+2])
		size := int(binary.BigEndian.Uint16(body[p+2 : p+4]))
		p += 4
		if p+size > layout.extEndBody {
			return layout, errors.New("ClientHello extension is truncated")
		}
		if kind == 0 && !layout.hasSNIExtension {
			layout.hasSNIExtension = true
			layout.sniStartBody = start
			layout.sniEndBody = p + size
		}
		p += size
	}
	if p != layout.extEndBody {
		return layout, errors.New("ClientHello extensions are malformed")
	}
	return layout, nil
}

func v2ClientHelloForgeInfoFor(data []byte) v2ClientHelloForgeInfo {
	base := v2ValidateClientHello(data)
	out := v2ClientHelloForgeInfo{
		Valid: base.Valid, SNI: base.SNI, Size: base.Size, Detail: base.Detail,
		ALPN: []string{}, SupportedVersions: []string{},
	}
	layout, err := v2ParseClientHelloLayout(data)
	if err != nil {
		if out.Detail == "" {
			out.Detail = err.Error()
		}
		out.Valid = false
		return out
	}
	body := data[layout.bodyStart:]
	p := layout.extStartBody
	for p+4 <= layout.extEndBody {
		kind := binary.BigEndian.Uint16(body[p : p+2])
		size := int(binary.BigEndian.Uint16(body[p+2 : p+4]))
		p += 4
		ext := body[p : p+size]
		out.ExtensionCount++
		switch kind {
		case 16:
			if len(ext) >= 2 {
				listLen := int(binary.BigEndian.Uint16(ext[:2]))
				q := 2
				end := q + listLen
				if end <= len(ext) {
					for q < end {
						n := int(ext[q])
						q++
						if q+n > end {
							break
						}
						out.ALPN = append(out.ALPN, string(ext[q:q+n]))
						q += n
					}
				}
			}
		case 43:
			if len(ext) >= 1 {
				q := 1
				end := q + int(ext[0])
				if end <= len(ext) {
					for q+2 <= end {
						v := binary.BigEndian.Uint16(ext[q : q+2])
						switch v {
						case 0x0304:
							out.SupportedVersions = append(out.SupportedVersions, "TLS 1.3")
						case 0x0303:
							out.SupportedVersions = append(out.SupportedVersions, "TLS 1.2")
						default:
							out.SupportedVersions = append(out.SupportedVersions, "0x"+strings.ToUpper(hex4(v)))
						}
						q += 2
					}
				}
			}
		}
		p += size
	}
	return out
}

func hex4(v uint16) string {
	const h = "0123456789abcdef"
	return string([]byte{h[(v>>12)&15], h[(v>>8)&15], h[(v>>4)&15], h[v&15]})
}

func v2BuildSNIExtension(sni string) []byte {
	name := []byte(sni)
	dataLen := 2 + 1 + 2 + len(name)
	out := make([]byte, 4+dataLen)
	binary.BigEndian.PutUint16(out[0:2], 0)
	binary.BigEndian.PutUint16(out[2:4], uint16(dataLen))
	binary.BigEndian.PutUint16(out[4:6], uint16(1+2+len(name)))
	out[6] = 0
	binary.BigEndian.PutUint16(out[7:9], uint16(len(name)))
	copy(out[9:], name)
	return out
}

func v2RewriteClientHelloSNI(data []byte, newSNI string) ([]byte, error) {
	normalized, err := v2NormalizeTarget(newSNI)
	if err != nil {
		return nil, err
	}
	layout, err := v2ParseClientHelloLayout(data)
	if err != nil {
		return nil, err
	}
	body := append([]byte(nil), data[layout.bodyStart:]...)
	newSNIExt := v2BuildSNIExtension(normalized)

	var newBody []byte
	if layout.hasSNIExtension {
		newBody = make([]byte, 0, len(body)-(layout.sniEndBody-layout.sniStartBody)+len(newSNIExt))
		newBody = append(newBody, body[:layout.sniStartBody]...)
		newBody = append(newBody, newSNIExt...)
		newBody = append(newBody, body[layout.sniEndBody:]...)
	} else {
		newBody = make([]byte, 0, len(body)+len(newSNIExt))
		newBody = append(newBody, body[:layout.extStartBody]...)
		newBody = append(newBody, newSNIExt...)
		newBody = append(newBody, body[layout.extStartBody:]...)
	}

	newExtLen := len(newBody) - layout.extStartBody
	if newExtLen < 0 || newExtLen > 0xffff {
		return nil, errors.New("rewritten ClientHello extensions are too large")
	}
	binary.BigEndian.PutUint16(newBody[layout.extLenPosBody:layout.extLenPosBody+2], uint16(newExtLen))
	if len(newBody) > 0xffffff {
		return nil, errors.New("rewritten ClientHello is too large")
	}

	out := make([]byte, 9+len(newBody))
	copy(out[:5], data[:5])
	out[5] = 0x01
	out[6] = byte(len(newBody) >> 16)
	out[7] = byte(len(newBody) >> 8)
	out[8] = byte(len(newBody))
	copy(out[9:], newBody)
	recordPayloadLen := len(out) - 5
	if recordPayloadLen > 0xffff {
		return nil, errors.New("rewritten TLS record is too large")
	}
	binary.BigEndian.PutUint16(out[3:5], uint16(recordPayloadLen))

	info := v2ClientHelloForgeInfoFor(out)
	if !info.Valid || !strings.EqualFold(info.SNI, normalized) {
		return nil, errors.New("rewritten ClientHello validation failed")
	}
	return out, nil
}

func handleV2ClientHelloForge(w http.ResponseWriter, r *http.Request) {
	var req v2ClientHelloForgeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid ClientHello forge request"})
		return
	}
	source, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "content_base64 is invalid"})
		return
	}
	sourceInfo := v2ClientHelloForgeInfoFor(source)
	if !sourceInfo.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "source ClientHello is invalid: " + sourceInfo.Detail})
		return
	}
	out, err := v2RewriteClientHelloSNI(source, req.SNI)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	info := v2ClientHelloForgeInfoFor(out)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "source": "captured-clone", "source_sni": sourceInfo.SNI,
		"sni": info.SNI, "size": info.Size, "valid": info.Valid, "detail": info.Detail,
		"sha256": blobSHA256(out), "content_base64": base64.StdEncoding.EncodeToString(out),
		"alpn": info.ALPN, "supported_versions": info.SupportedVersions,
		"extension_count": info.ExtensionCount, "fingerprint_preserved": true,
	})
}

func v2ClientHelloLuaPaths() []string {
	return []string{
		"/opt/etc/nfqws2/lua/zapret-antidpi.lua",
		"/opt/share/nfqws2/lua/zapret-antidpi.lua",
		"/opt/share/zapret2/lua/zapret-antidpi.lua",
		"/opt/zapret2/lua/zapret-antidpi.lua",
		"/opt/zapret2/files/lua/zapret-antidpi.lua",
	}
}

func v2ClientHelloDynamicCloneCapability() (bool, string) {
	for _, path := range v2ClientHelloLuaPaths() {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if strings.Contains(text, "function tls_client_hello_clone") &&
			strings.Contains(text, "tls_client_hello_mod") &&
			strings.Contains(text, "desync.reasm_data") {
			return true, path
		}
	}
	return false, ""
}

func v2TCPDumpAvailable() bool {
	for _, path := range []string{"/opt/sbin/tcpdump", "/opt/bin/tcpdump", "/usr/sbin/tcpdump", "/usr/bin/tcpdump"} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return true
		}
	}
	return false
}

func handleV2ClientHelloCapabilities(w http.ResponseWriter, _ *http.Request) {
	dynamic, source := v2ClientHelloDynamicCloneCapability()
	writeJSON(w, http.StatusOK, v2ClientHelloCapability{
		DynamicCloneAvailable: dynamic,
		DynamicCloneSource:    source,
		StaticForgeAvailable:  true,
		CaptureAvailable:      v2TCPDumpAvailable(),
		TCPDumpAvailable:      v2TCPDumpAvailable(),
	})
}

type v2RobustCaptureFlow struct {
	src, dst string
	dport    int
	segments map[uint32][]byte
	startSeq *uint32
}

func v2ParsePCAPClientHellosRobust(data []byte) ([]v2ClientHelloCandidate, error) {
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
	flows := map[string]*v2RobustCaptureFlow{}
	seenSHA := map[string]bool{}
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
		if !ok || len(payload) == 0 {
			continue
		}
		key := src + "|" + sport + ">" + dst + "|" + itoaPort(dport)
		f := flows[key]
		if f == nil {
			if len(flows) >= 128 {
				continue
			}
			f = &v2RobustCaptureFlow{src: src, dst: dst, dport: dport, segments: map[uint32][]byte{}}
			flows[key] = f
		}
		if len(f.segments) < 64 {
			if old, exists := f.segments[seq]; !exists || len(payload) > len(old) {
				f.segments[seq] = append([]byte(nil), payload...)
			}
		}
		if f.startSeq == nil && len(payload) >= 9 && payload[0] == 0x16 && payload[1] == 0x03 && payload[5] == 0x01 {
			v := seq
			f.startSeq = &v
		}
		if f.startSeq == nil {
			continue
		}
		record, complete := v2AssembleTLSRecord(f)
		if !complete {
			continue
		}
		delete(flows, key)
		info := v2ValidateClientHello(record)
		if !info.Valid {
			continue
		}
		hash := blobSHA256(record)
		if seenSHA[hash] {
			continue
		}
		seenSHA[hash] = true
		out = append(out, v2ClientHelloCandidate{
			SrcIP: f.src, DstIP: f.dst, DstPort: f.dport,
			SNI: info.SNI, Size: len(record), Valid: true, Detail: info.Detail,
			SHA256: hash, ContentBase64: base64.StdEncoding.EncodeToString(record),
		})
		if len(out) >= v2ClientHelloCaptureMaxItems {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SNI != out[j].SNI {
			return strings.ToLower(out[i].SNI) < strings.ToLower(out[j].SNI)
		}
		return out[i].SHA256 < out[j].SHA256
	})
	return out, nil
}

func v2AssembleTLSRecord(f *v2RobustCaptureFlow) ([]byte, bool) {
	if f == nil || f.startSeq == nil {
		return nil, false
	}
	type seg struct {
		delta int64
		data  []byte
	}
	items := make([]seg, 0, len(f.segments))
	for seq, data := range f.segments {
		delta := int64(int32(seq - *f.startSeq))
		if delta < 0 || delta > 1<<20 {
			continue
		}
		items = append(items, seg{delta: delta, data: data})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].delta < items[j].delta })
	if len(items) == 0 || items[0].delta != 0 {
		return nil, false
	}
	buf := make([]byte, 0, 4096)
	need := 0
	for _, item := range items {
		if item.delta > int64(len(buf)) {
			return nil, false
		}
		start := 0
		if item.delta < int64(len(buf)) {
			start = len(buf) - int(item.delta)
			if start >= len(item.data) {
				continue
			}
		}
		buf = append(buf, item.data[start:]...)
		if need == 0 && len(buf) >= 5 {
			if buf[0] != 0x16 || buf[1] != 0x03 {
				return nil, false
			}
			need = 5 + int(binary.BigEndian.Uint16(buf[3:5]))
			if need < 9 || need > 64<<10 {
				return nil, false
			}
		}
		if need > 0 && len(buf) >= need {
			return append([]byte(nil), buf[:need]...), true
		}
	}
	return nil, false
}

func itoaPort(v int) string {
	if v == 0 {
		return "0"
	}
	var b [6]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}

func v2ClientHelloBlobID(name string) string {
	base := strings.TrimSuffix(filepath.Base(strings.ToLower(strings.TrimSpace(name))), filepath.Ext(name))
	var b strings.Builder
	b.WriteString("rf_")
	for _, r := range base {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
