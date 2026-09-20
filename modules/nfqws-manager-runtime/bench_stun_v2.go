package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	benchSTUNHeaderSize       = 20
	benchSTUNMagicCookie      = uint32(0x2112a442)
	benchSTUNBindingRequest   = uint16(0x0001)
	benchSTUNBindingSuccess   = uint16(0x0101)
	benchSTUNAttrMapped       = uint16(0x0001)
	benchSTUNAttrXORMapped    = uint16(0x0020)
	benchSTUNTransactionIDLen = 12
)

type benchSTUNRequest struct {
	Packet        []byte
	TransactionID []byte
}

type benchSTUNResponse struct {
	MessageType        string
	TransactionMatched bool
	MappedAddress      string
	MappedPort         int
}

func benchSTUNBuildBindingRequest() (benchSTUNRequest, error) {
	transactionID := make([]byte, benchSTUNTransactionIDLen)
	if _, err := rand.Read(transactionID); err != nil {
		return benchSTUNRequest{}, err
	}

	packet := make([]byte, benchSTUNHeaderSize)
	binary.BigEndian.PutUint16(packet[0:2], benchSTUNBindingRequest)
	binary.BigEndian.PutUint16(packet[2:4], 0)
	binary.BigEndian.PutUint32(packet[4:8], benchSTUNMagicCookie)
	copy(packet[8:20], transactionID)

	return benchSTUNRequest{Packet: packet, TransactionID: transactionID}, nil
}

func benchSTUNMappedAddress(attributeType uint16, value, transactionID []byte) (string, int, error) {
	if len(value) < 4 {
		return "", 0, errors.New("STUN mapped-address attribute is too short")
	}
	if value[0] != 0 {
		return "", 0, errors.New("STUN mapped-address reserved byte is non-zero")
	}

	port := binary.BigEndian.Uint16(value[2:4])
	if attributeType == benchSTUNAttrXORMapped {
		port ^= uint16(benchSTUNMagicCookie >> 16)
	}
	if port == 0 {
		return "", 0, errors.New("STUN mapped-address port is zero")
	}

	switch value[1] {
	case 0x01:
		if len(value) != 8 {
			return "", 0, errors.New("invalid STUN IPv4 mapped-address length")
		}
		address := append([]byte{}, value[4:8]...)
		if attributeType == benchSTUNAttrXORMapped {
			cookie := make([]byte, 4)
			binary.BigEndian.PutUint32(cookie, benchSTUNMagicCookie)
			for i := range address {
				address[i] ^= cookie[i]
			}
		}
		ip := net.IP(address).To4()
		if ip == nil {
			return "", 0, errors.New("invalid STUN IPv4 mapped address")
		}
		return ip.String(), int(port), nil
	case 0x02:
		if len(value) != 20 {
			return "", 0, errors.New("invalid STUN IPv6 mapped-address length")
		}
		address := append([]byte{}, value[4:20]...)
		if attributeType == benchSTUNAttrXORMapped {
			mask := make([]byte, 16)
			binary.BigEndian.PutUint32(mask[0:4], benchSTUNMagicCookie)
			if len(transactionID) != benchSTUNTransactionIDLen {
				return "", 0, errors.New("invalid STUN transaction ID length")
			}
			copy(mask[4:], transactionID)
			for i := range address {
				address[i] ^= mask[i]
			}
		}
		ip := net.IP(address)
		if ip.To16() == nil {
			return "", 0, errors.New("invalid STUN IPv6 mapped address")
		}
		return ip.String(), int(port), nil
	default:
		return "", 0, errors.New("unsupported STUN mapped-address family")
	}
}

func benchSTUNParseBindingResponse(packet, transactionID []byte) (benchSTUNResponse, error) {
	if len(transactionID) != benchSTUNTransactionIDLen {
		return benchSTUNResponse{}, errors.New("invalid STUN transaction ID length")
	}
	if len(packet) < benchSTUNHeaderSize {
		return benchSTUNResponse{}, errors.New("STUN response is shorter than the header")
	}
	if packet[0]&0xc0 != 0 {
		return benchSTUNResponse{}, errors.New("STUN response has invalid leading bits")
	}
	if binary.BigEndian.Uint32(packet[4:8]) != benchSTUNMagicCookie {
		return benchSTUNResponse{}, errors.New("STUN magic cookie mismatch")
	}

	messageLength := int(binary.BigEndian.Uint16(packet[2:4]))
	if messageLength%4 != 0 || benchSTUNHeaderSize+messageLength != len(packet) {
		return benchSTUNResponse{}, errors.New("STUN message length is invalid")
	}
	if !hmac.Equal(packet[8:20], transactionID) {
		return benchSTUNResponse{}, errors.New("STUN transaction ID mismatch")
	}

	messageType := binary.BigEndian.Uint16(packet[0:2])
	if messageType != benchSTUNBindingSuccess {
		return benchSTUNResponse{}, fmt.Errorf("unexpected STUN response type 0x%04x", messageType)
	}

	response := benchSTUNResponse{
		MessageType:        "binding-success",
		TransactionMatched: true,
	}

	position := benchSTUNHeaderSize
	end := benchSTUNHeaderSize + messageLength
	for position < end {
		if position+4 > end {
			return benchSTUNResponse{}, errors.New("truncated STUN attribute header")
		}
		attributeType := binary.BigEndian.Uint16(packet[position : position+2])
		attributeLength := int(binary.BigEndian.Uint16(packet[position+2 : position+4]))
		valueStart := position + 4
		valueEnd := valueStart + attributeLength
		if valueEnd > end {
			return benchSTUNResponse{}, errors.New("truncated STUN attribute value")
		}

		if attributeType == benchSTUNAttrXORMapped || attributeType == benchSTUNAttrMapped {
			mappedAddress, mappedPort, err := benchSTUNMappedAddress(attributeType, packet[valueStart:valueEnd], transactionID)
			if err == nil {
				response.MappedAddress = mappedAddress
				response.MappedPort = mappedPort
				return response, nil
			}
		}

		paddedLength := (attributeLength + 3) &^ 3
		position = valueStart + paddedLength
	}

	return benchSTUNResponse{}, errors.New("STUN Binding success response has no valid mapped-address attribute")
}

func v2ProbeSTUN(ctx context.Context, ip string, localPort, remotePort int) (v2HTTPMetrics, error) {
	metrics := v2HTTPMetrics{}
	request, err := benchSTUNBuildBindingRequest()
	if err != nil {
		return metrics, fmt.Errorf("build STUN Binding request: %w", err)
	}

	remote := &net.UDPAddr{IP: net.ParseIP(ip).To4(), Port: remotePort}
	if remote.IP == nil {
		return metrics, errors.New("invalid STUN destination IPv4")
	}
	local := &net.UDPAddr{IP: net.IPv4zero, Port: localPort}
	conn, err := net.DialUDP("udp4", local, remote)
	if err != nil {
		return metrics, fmt.Errorf("udp connect: %w", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(6 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return metrics, fmt.Errorf("udp deadline: %w", err)
	}

	started := time.Now()
	written, err := conn.Write(request.Packet)
	metrics.UDPWriteBytes = int64(written)
	if err != nil {
		return metrics, fmt.Errorf("send STUN Binding request: %w", err)
	}
	if written != len(request.Packet) {
		return metrics, fmt.Errorf("short STUN Binding write: %d/%d", written, len(request.Packet))
	}

	buffer := make([]byte, 4096)
	for metrics.UDPReadDatagrams < 8 {
		n, readErr := conn.Read(buffer)
		if readErr != nil {
			metrics.DurationMS = time.Since(started).Milliseconds()
			if metrics.UDPReadDatagrams > 0 {
				return metrics, errors.New("no valid correlated STUN Binding response was proven")
			}
			return metrics, fmt.Errorf("read STUN response: %w", readErr)
		}
		if metrics.UDPReadDatagrams == 0 {
			metrics.TTFBMS = time.Since(started).Milliseconds()
		}
		metrics.UDPReadDatagrams++
		metrics.UDPReadBytes += int64(n)

		response, parseErr := benchSTUNParseBindingResponse(buffer[:n], request.TransactionID)
		if parseErr != nil || !response.TransactionMatched {
			continue
		}

		metrics.STUNTransactionMatched = true
		metrics.STUNMessageType = response.MessageType
		metrics.STUNMappedAddress = response.MappedAddress
		metrics.STUNMappedPort = response.MappedPort
		metrics.STUNResponseProven = true
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
	return metrics, errors.New("STUN response limit reached without protocol evidence")
}
