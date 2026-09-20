package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	benchQUICVersion1       = uint32(1)
	benchQUICMinInitialSize = 1200
	benchQUICCIDLength      = 8
	benchQUICPacketNumber   = uint32(0)
)

// QUIC v1 Initial salt from RFC 9001 section 5.2.
var benchQUICV1InitialSalt = []byte{
	0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17,
	0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a,
}

type benchQUICInitial struct {
	Packet []byte
	DCID   []byte
	SCID   []byte
}

func benchQUICAppendVarint(dst []byte, value uint64) ([]byte, error) {
	switch {
	case value <= 63:
		return append(dst, byte(value)), nil
	case value <= 16383:
		var raw [2]byte
		binary.BigEndian.PutUint16(raw[:], uint16(value)|0x4000)
		return append(dst, raw[:]...), nil
	case value <= 1073741823:
		var raw [4]byte
		binary.BigEndian.PutUint32(raw[:], uint32(value)|0x80000000)
		return append(dst, raw[:]...), nil
	case value <= 4611686018427387903:
		var raw [8]byte
		binary.BigEndian.PutUint64(raw[:], value|0xc000000000000000)
		return append(dst, raw[:]...), nil
	default:
		return nil, errors.New("QUIC varint exceeds 62-bit range")
	}
}

func benchQUICHKDFExtract(salt, input []byte) []byte {
	mac := hmac.New(sha256.New, salt)
	_, _ = mac.Write(input)
	return mac.Sum(nil)
}

func benchQUICHKDFExpand(secret, info []byte, length int) ([]byte, error) {
	if length <= 0 || length > 255*sha256.Size {
		return nil, errors.New("invalid HKDF output length")
	}
	out := make([]byte, 0, length)
	var previous []byte
	for counter := byte(1); len(out) < length; counter++ {
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write(previous)
		_, _ = mac.Write(info)
		_, _ = mac.Write([]byte{counter})
		previous = mac.Sum(nil)
		need := length - len(out)
		if need > len(previous) {
			need = len(previous)
		}
		out = append(out, previous[:need]...)
		if counter == 255 && len(out) < length {
			return nil, errors.New("HKDF expansion exceeded block limit")
		}
	}
	return out, nil
}

func benchQUICHKDFExpandLabel(secret []byte, label string, length int) ([]byte, error) {
	fullLabel := []byte("tls13 " + label)
	if len(fullLabel) > 255 || length > 65535 {
		return nil, errors.New("invalid HKDF label")
	}
	info := make([]byte, 0, 4+len(fullLabel))
	var size [2]byte
	binary.BigEndian.PutUint16(size[:], uint16(length))
	info = append(info, size[:]...)
	info = append(info, byte(len(fullLabel)))
	info = append(info, fullLabel...)
	info = append(info, 0)
	return benchQUICHKDFExpand(secret, info, length)
}

func benchQUICAppendTLSExtension(dst []byte, extensionType uint16, data []byte) []byte {
	var header [4]byte
	binary.BigEndian.PutUint16(header[0:2], extensionType)
	binary.BigEndian.PutUint16(header[2:4], uint16(len(data)))
	dst = append(dst, header[:]...)
	return append(dst, data...)
}

func benchQUICTransportParameters(scid []byte) ([]byte, error) {
	out := []byte{}
	appendParam := func(id uint64, value []byte) error {
		var err error
		out, err = benchQUICAppendVarint(out, id)
		if err != nil {
			return err
		}
		out, err = benchQUICAppendVarint(out, uint64(len(value)))
		if err != nil {
			return err
		}
		out = append(out, value...)
		return nil
	}
	encodeValue := func(value uint64) ([]byte, error) {
		return benchQUICAppendVarint(nil, value)
	}

	maxPayload, err := encodeValue(1200)
	if err != nil {
		return nil, err
	}
	if err := appendParam(0x03, maxPayload); err != nil {
		return nil, err
	}
	if err := appendParam(0x0f, append([]byte{}, scid...)); err != nil {
		return nil, err
	}
	activeLimit, err := encodeValue(2)
	if err != nil {
		return nil, err
	}
	if err := appendParam(0x0e, activeLimit); err != nil {
		return nil, err
	}
	return out, nil
}

func benchQUICTLSClientHello(serverName string, scid []byte) ([]byte, error) {
	host, err := normalizeBenchServerName(serverName)
	if err != nil {
		return nil, err
	}

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, err
	}

	privateKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	publicKey := privateKey.PublicKey().Bytes()

	extensions := []byte{}

	hostBytes := []byte(host)
	serverNameData := make([]byte, 0, 5+len(hostBytes))
	serverNameListLength := 3 + len(hostBytes)
	serverNameData = append(serverNameData, byte(serverNameListLength>>8), byte(serverNameListLength))
	serverNameData = append(serverNameData, 0)
	serverNameData = append(serverNameData, byte(len(hostBytes)>>8), byte(len(hostBytes)))
	serverNameData = append(serverNameData, hostBytes...)
	extensions = benchQUICAppendTLSExtension(extensions, 0x0000, serverNameData)

	extensions = benchQUICAppendTLSExtension(extensions, 0x000a, []byte{0x00, 0x02, 0x00, 0x1d})
	extensions = benchQUICAppendTLSExtension(extensions, 0x000d, []byte{
		0x00, 0x06,
		0x08, 0x04,
		0x04, 0x03,
		0x08, 0x07,
	})
	extensions = benchQUICAppendTLSExtension(extensions, 0x002b, []byte{0x02, 0x03, 0x04})

	keyShare := make([]byte, 0, 6+len(publicKey))
	entryLength := 4 + len(publicKey)
	keyShare = append(keyShare, byte(entryLength>>8), byte(entryLength))
	keyShare = append(keyShare, 0x00, 0x1d)
	keyShare = append(keyShare, byte(len(publicKey)>>8), byte(len(publicKey)))
	keyShare = append(keyShare, publicKey...)
	extensions = benchQUICAppendTLSExtension(extensions, 0x0033, keyShare)

	alpn := []byte{0x00, 0x03, 0x02, 'h', '3'}
	extensions = benchQUICAppendTLSExtension(extensions, 0x0010, alpn)

	transportParameters, err := benchQUICTransportParameters(scid)
	if err != nil {
		return nil, err
	}
	extensions = benchQUICAppendTLSExtension(extensions, 0x0039, transportParameters)

	body := make([]byte, 0, 128+len(hostBytes))
	body = append(body, 0x03, 0x03)
	body = append(body, randomBytes...)
	body = append(body, 0)
	body = append(body, 0x00, 0x04, 0x13, 0x01, 0x13, 0x02)
	body = append(body, 0x01, 0x00)
	body = append(body, byte(len(extensions)>>8), byte(len(extensions)))
	body = append(body, extensions...)

	if len(body) > 0xffffff {
		return nil, errors.New("TLS ClientHello exceeds handshake length")
	}
	handshake := []byte{
		0x01,
		byte(len(body) >> 16),
		byte(len(body) >> 8),
		byte(len(body)),
	}
	handshake = append(handshake, body...)
	return handshake, nil
}

// benchQUICBuildInitial emits a real QUIC v1 Initial carrying a TLS ClientHello.
// It is intentionally limited to transport evidence; it is not an HTTP/3 client.
func benchQUICBuildInitial(serverName string) (benchQUICInitial, error) {
	dcid := make([]byte, benchQUICCIDLength)
	scid := make([]byte, benchQUICCIDLength)
	if _, err := rand.Read(dcid); err != nil {
		return benchQUICInitial{}, err
	}
	if _, err := rand.Read(scid); err != nil {
		return benchQUICInitial{}, err
	}

	clientHello, err := benchQUICTLSClientHello(serverName, scid)
	if err != nil {
		return benchQUICInitial{}, err
	}

	cryptoFrame := []byte{0x06, 0x00}
	cryptoFrame, err = benchQUICAppendVarint(cryptoFrame, uint64(len(clientHello)))
	if err != nil {
		return benchQUICInitial{}, err
	}
	cryptoFrame = append(cryptoFrame, clientHello...)

	headerPrefix := []byte{0xc3, 0x00, 0x00, 0x00, 0x01, byte(len(dcid))}
	headerPrefix = append(headerPrefix, dcid...)
	headerPrefix = append(headerPrefix, byte(len(scid)))
	headerPrefix = append(headerPrefix, scid...)
	headerPrefix = append(headerPrefix, 0x00)

	const packetNumberLength = 4
	const authTagLength = 16
	const lengthFieldSize = 2
	plaintextLength := benchQUICMinInitialSize - len(headerPrefix) - lengthFieldSize - packetNumberLength - authTagLength
	if plaintextLength < len(cryptoFrame) {
		return benchQUICInitial{}, errors.New("QUIC ClientHello does not fit minimum Initial datagram")
	}
	plaintext := append([]byte{}, cryptoFrame...)
	plaintext = append(plaintext, make([]byte, plaintextLength-len(plaintext))...)

	lengthValue := uint64(packetNumberLength + plaintextLength + authTagLength)
	lengthField, err := benchQUICAppendVarint(nil, lengthValue)
	if err != nil {
		return benchQUICInitial{}, err
	}
	if len(lengthField) != lengthFieldSize {
		return benchQUICInitial{}, errors.New("unexpected QUIC Initial length-field size")
	}

	packetNumber := make([]byte, packetNumberLength)
	binary.BigEndian.PutUint32(packetNumber, benchQUICPacketNumber)

	header := append([]byte{}, headerPrefix...)
	header = append(header, lengthField...)
	pnOffset := len(header)
	header = append(header, packetNumber...)

	initialSecret := benchQUICHKDFExtract(benchQUICV1InitialSalt, dcid)
	clientSecret, err := benchQUICHKDFExpandLabel(initialSecret, "client in", sha256.Size)
	if err != nil {
		return benchQUICInitial{}, err
	}
	key, err := benchQUICHKDFExpandLabel(clientSecret, "quic key", 16)
	if err != nil {
		return benchQUICInitial{}, err
	}
	iv, err := benchQUICHKDFExpandLabel(clientSecret, "quic iv", 12)
	if err != nil {
		return benchQUICInitial{}, err
	}
	hpKey, err := benchQUICHKDFExpandLabel(clientSecret, "quic hp", 16)
	if err != nil {
		return benchQUICInitial{}, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return benchQUICInitial{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return benchQUICInitial{}, err
	}
	nonce := append([]byte{}, iv...)
	pn64 := uint64(benchQUICPacketNumber)
	for i := 0; i < 8; i++ {
		nonce[len(nonce)-1-i] ^= byte(pn64 >> (8 * i))
	}
	ciphertext := aead.Seal(nil, nonce, plaintext, header)

	packet := append([]byte{}, header...)
	packet = append(packet, ciphertext...)
	if len(packet) != benchQUICMinInitialSize {
		return benchQUICInitial{}, fmt.Errorf("unexpected QUIC Initial size %d", len(packet))
	}

	sampleOffset := pnOffset + 4
	if sampleOffset+aes.BlockSize > len(packet) {
		return benchQUICInitial{}, errors.New("QUIC header-protection sample is unavailable")
	}
	hpBlock, err := aes.NewCipher(hpKey)
	if err != nil {
		return benchQUICInitial{}, err
	}
	mask := make([]byte, aes.BlockSize)
	hpBlock.Encrypt(mask, packet[sampleOffset:sampleOffset+aes.BlockSize])
	packet[0] ^= mask[0] & 0x0f
	for i := 0; i < packetNumberLength; i++ {
		packet[pnOffset+i] ^= mask[i+1]
	}

	return benchQUICInitial{Packet: packet, DCID: dcid, SCID: scid}, nil
}

// A response is accepted as protocol evidence only when it is a valid long-header
// QUIC v1/version-negotiation packet addressed to the client SCID we generated.
func benchQUICParseLongHeaderResponse(packet, clientSCID []byte) (string, string, bool, error) {
	if len(packet) < 7 || packet[0]&0x80 == 0 {
		return "", "", false, errors.New("response is not a QUIC long-header packet")
	}
	version := binary.BigEndian.Uint32(packet[1:5])
	if version != 0 && version != benchQUICVersion1 {
		return "", fmt.Sprintf("0x%08x", version), false, errors.New("unexpected QUIC response version")
	}
	if version != 0 && packet[0]&0x40 == 0 {
		return "", fmt.Sprintf("0x%08x", version), false, errors.New("QUIC fixed bit is not set")
	}
	position := 5
	dcidLength := int(packet[position])
	position++
	if dcidLength > 20 || position+dcidLength+1 > len(packet) {
		return "", "", false, errors.New("invalid QUIC destination connection id")
	}
	dcid := packet[position : position+dcidLength]
	position += dcidLength
	scidLength := int(packet[position])
	position++
	if scidLength > 20 || position+scidLength > len(packet) {
		return "", "", false, errors.New("invalid QUIC source connection id")
	}
	position += scidLength

	cidMatched := hmac.Equal(dcid, clientSCID)
	versionText := fmt.Sprintf("0x%08x", version)
	if version == 0 {
		if len(packet)-position < 4 || (len(packet)-position)%4 != 0 {
			return "", versionText, cidMatched, errors.New("invalid QUIC version-negotiation payload")
		}
		return "version-negotiation", versionText, cidMatched, nil
	}

	packetType := (packet[0] >> 4) & 0x03
	responseType := "unknown-long"
	if version == benchQUICVersion1 {
		switch packetType {
		case 0:
			responseType = "initial"
		case 1:
			responseType = "0rtt"
		case 2:
			responseType = "handshake"
		case 3:
			responseType = "retry"
		}
	}
	return responseType, versionText, cidMatched, nil
}

func v2ProbeQUIC(ctx context.Context, host, ip string, localPort, remotePort int) (v2HTTPMetrics, error) {
	metrics := v2HTTPMetrics{}
	initial, err := benchQUICBuildInitial(host)
	if err != nil {
		return metrics, fmt.Errorf("build QUIC Initial: %w", err)
	}

	remote := &net.UDPAddr{IP: net.ParseIP(ip).To4(), Port: remotePort}
	local := &net.UDPAddr{IP: net.IPv4zero, Port: localPort}
	conn, err := net.DialUDP("udp4", local, remote)
	if err != nil {
		return metrics, fmt.Errorf("udp connect: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(8 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return metrics, fmt.Errorf("udp deadline: %w", err)
	}

	started := time.Now()
	written, err := conn.Write(initial.Packet)
	metrics.UDPWriteBytes = int64(written)
	if err != nil {
		return metrics, fmt.Errorf("send QUIC Initial: %w", err)
	}
	if written != len(initial.Packet) {
		return metrics, fmt.Errorf("short QUIC Initial write: %d/%d", written, len(initial.Packet))
	}

	buffer := make([]byte, 64<<10)
	for metrics.UDPReadDatagrams < 8 {
		n, readErr := conn.Read(buffer)
		if readErr != nil {
			metrics.DurationMS = time.Since(started).Milliseconds()
			if metrics.UDPReadDatagrams > 0 {
				return metrics, errors.New("no valid QUIC long-header response was proven")
			}
			return metrics, fmt.Errorf("read QUIC response: %w", readErr)
		}
		if metrics.UDPReadDatagrams == 0 {
			metrics.TTFBMS = time.Since(started).Milliseconds()
		}
		metrics.UDPReadDatagrams++
		metrics.UDPReadBytes += int64(n)

		responseType, version, cidMatched, parseErr := benchQUICParseLongHeaderResponse(buffer[:n], initial.SCID)
		if parseErr != nil || !cidMatched {
			continue
		}
		metrics.QUICLongHeader = true
		metrics.QUICCIDMatched = true
		metrics.QUICVersion = version
		metrics.QUICResponseType = responseType
		metrics.QUICResponseProven = true
		metrics.ResponseComplete = true
		metrics.ProgressProven = true
		metrics.Bytes = metrics.UDPReadBytes
		metrics.DurationMS = time.Since(started).Milliseconds()
		if metrics.DurationMS > 0 {
			metrics.ThroughputBPS = (metrics.Bytes * 1000) / metrics.DurationMS
		}
		return metrics, nil
	}

	metrics.DurationMS = time.Since(started).Milliseconds()
	return metrics, errors.New("QUIC response limit reached without protocol evidence")
}
