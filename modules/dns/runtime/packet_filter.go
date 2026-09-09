package main

import "fmt"

const (
	bpfClassLD  = 0x00
	bpfClassLDX = 0x01
	bpfClassJMP = 0x05
	bpfClassRET = 0x06

	bpfSizeH = 0x08
	bpfSizeB = 0x10

	bpfModeABS = 0x20
	bpfModeIND = 0x40
	bpfModeMSH = 0xa0

	bpfOpJA  = 0x00
	bpfOpJEQ = 0x10
	bpfSrcK  = 0x00

	etherTypeIPv4 = 0x0800
	etherTypeIPv6 = 0x86dd
	etherTypeVLAN = 0x8100
	etherTypeQinQ = 0x88a8

	ipProtoTCP = 6
	ipProtoUDP = 17

	packetFilterReject = 0
	packetFilterAccept = 0xffffffff
)

type classicBPFInstruction struct {
	Code uint16
	Jt   uint8
	Jf   uint8
	K    uint32
}

type bpfFixup struct {
	index      int
	trueLabel  string
	falseLabel string
	jumpLabel  string
}

type bpfBuilder struct {
	instructions []classicBPFInstruction
	labels       map[string]int
	fixups       []bpfFixup
}

func newBPFBuilder() *bpfBuilder {
	return &bpfBuilder{labels: make(map[string]int)}
}

func (b *bpfBuilder) label(name string) {
	if _, exists := b.labels[name]; exists {
		panic("duplicate BPF label: " + name)
	}
	b.labels[name] = len(b.instructions)
}

func (b *bpfBuilder) stmt(code uint16, k uint32) {
	b.instructions = append(b.instructions, classicBPFInstruction{Code: code, K: k})
}

func (b *bpfBuilder) loadHalfAbs(offset uint32) {
	b.stmt(bpfClassLD|bpfSizeH|bpfModeABS, offset)
}

func (b *bpfBuilder) loadByteAbs(offset uint32) {
	b.stmt(bpfClassLD|bpfSizeB|bpfModeABS, offset)
}

func (b *bpfBuilder) loadXMSH(offset uint32) {
	b.stmt(bpfClassLDX|bpfSizeB|bpfModeMSH, offset)
}

func (b *bpfBuilder) loadHalfInd(offset uint32) {
	b.stmt(bpfClassLD|bpfSizeH|bpfModeIND, offset)
}

func (b *bpfBuilder) jumpEqual(k uint32, trueLabel, falseLabel string) {
	index := len(b.instructions)
	b.instructions = append(b.instructions, classicBPFInstruction{
		Code: bpfClassJMP | bpfOpJEQ | bpfSrcK,
		K:    k,
	})
	b.fixups = append(b.fixups, bpfFixup{index: index, trueLabel: trueLabel, falseLabel: falseLabel})
}

func (b *bpfBuilder) jump(label string) {
	index := len(b.instructions)
	b.instructions = append(b.instructions, classicBPFInstruction{Code: bpfClassJMP | bpfOpJA})
	b.fixups = append(b.fixups, bpfFixup{index: index, jumpLabel: label})
}

func (b *bpfBuilder) ret(k uint32) {
	b.stmt(bpfClassRET|bpfSrcK, k)
}

func (b *bpfBuilder) finish() []classicBPFInstruction {
	for _, fixup := range b.fixups {
		if fixup.jumpLabel != "" {
			target, ok := b.labels[fixup.jumpLabel]
			if !ok {
				panic("unknown BPF label: " + fixup.jumpLabel)
			}
			delta := target - fixup.index - 1
			if delta < 0 {
				panic(fmt.Sprintf("backward classic BPF jump from %d to %d", fixup.index, target))
			}
			b.instructions[fixup.index].K = uint32(delta)
			continue
		}

		trueTarget, ok := b.labels[fixup.trueLabel]
		if !ok {
			panic("unknown BPF label: " + fixup.trueLabel)
		}
		falseTarget, ok := b.labels[fixup.falseLabel]
		if !ok {
			panic("unknown BPF label: " + fixup.falseLabel)
		}
		trueDelta := trueTarget - fixup.index - 1
		falseDelta := falseTarget - fixup.index - 1
		if trueDelta < 0 || trueDelta > 255 || falseDelta < 0 || falseDelta > 255 {
			panic(fmt.Sprintf("classic BPF conditional jump out of range at %d", fixup.index))
		}
		b.instructions[fixup.index].Jt = uint8(trueDelta)
		b.instructions[fixup.index].Jf = uint8(falseDelta)
	}

	return append([]classicBPFInstruction(nil), b.instructions...)
}

var proxyCaptureFilterProgram = buildProxyCaptureFilter()
var clientCaptureFilterProgram = buildClientCaptureFilter()

func buildProxyCaptureFilter() []classicBPFInstruction {
	b := newBPFBuilder()

	b.loadHalfAbs(12)
	b.jumpEqual(etherTypeIPv4, "ipv4", "check_ipv6")
	b.label("check_ipv6")
	b.jumpEqual(etherTypeIPv6, "ipv6", "reject")

	b.label("ipv4")
	b.loadByteAbs(14 + 9)
	b.jumpEqual(ipProtoUDP, "accept", "reject")

	b.label("ipv6")
	b.loadByteAbs(14 + 6)
	b.jumpEqual(ipProtoUDP, "accept", "reject")

	b.label("accept")
	b.ret(packetFilterAccept)
	b.label("reject")
	b.ret(packetFilterReject)

	return b.finish()
}

func buildClientCaptureFilter() []classicBPFInstruction {
	b := newBPFBuilder()

	b.loadHalfAbs(12)
	b.jumpEqual(etherTypeIPv4, "ipv4_0", "check_ipv6_0")
	b.label("check_ipv6_0")
	b.jumpEqual(etherTypeIPv6, "ipv6_0", "check_vlan_0")
	b.label("check_vlan_0")
	b.jumpEqual(etherTypeVLAN, "vlan_1", "check_qinq_0")
	b.label("check_qinq_0")
	b.jumpEqual(etherTypeQinQ, "vlan_1", "reject")

	b.label("vlan_1")
	b.loadHalfAbs(16)
	b.jumpEqual(etherTypeIPv4, "ipv4_1", "check_ipv6_1")
	b.label("check_ipv6_1")
	b.jumpEqual(etherTypeIPv6, "ipv6_1", "check_vlan_1")
	b.label("check_vlan_1")
	b.jumpEqual(etherTypeVLAN, "vlan_2", "check_qinq_1")
	b.label("check_qinq_1")
	b.jumpEqual(etherTypeQinQ, "vlan_2", "reject")

	b.label("vlan_2")
	b.loadHalfAbs(20)
	b.jumpEqual(etherTypeIPv4, "ipv4_2", "check_ipv6_2")
	b.label("check_ipv6_2")
	b.jumpEqual(etherTypeIPv6, "ipv6_2", "check_vlan_2")
	b.label("check_vlan_2")
	b.jumpEqual(etherTypeVLAN, "accept", "check_qinq_2")
	b.label("check_qinq_2")
	b.jumpEqual(etherTypeQinQ, "accept", "reject")

	emitIPv4DNSFilter(b, "ipv4_0", 14, "v4_0")
	emitIPv4DNSFilter(b, "ipv4_1", 18, "v4_1")
	emitIPv4DNSFilter(b, "ipv4_2", 22, "v4_2")
	emitIPv6DNSFilter(b, "ipv6_0", 14, "v6_0")
	emitIPv6DNSFilter(b, "ipv6_1", 18, "v6_1")
	emitIPv6DNSFilter(b, "ipv6_2", 22, "v6_2")

	b.label("accept")
	b.ret(packetFilterAccept)
	b.label("reject")
	b.ret(packetFilterReject)

	return b.finish()
}

func emitIPv4DNSFilter(b *bpfBuilder, label string, ipOffset uint32, prefix string) {
	portsLabel := prefix + "_ports"
	checkTCPLabel := prefix + "_check_tcp"
	checkDstLabel := prefix + "_check_dst"

	b.label(label)
	b.loadByteAbs(ipOffset + 9)
	b.jumpEqual(ipProtoUDP, portsLabel, checkTCPLabel)
	b.label(checkTCPLabel)
	b.jumpEqual(ipProtoTCP, portsLabel, "reject")

	b.label(portsLabel)
	b.loadXMSH(ipOffset)
	b.loadHalfInd(ipOffset)
	b.jumpEqual(53, "accept", checkDstLabel)
	b.label(checkDstLabel)
	b.loadHalfInd(ipOffset + 2)
	b.jumpEqual(53, "accept", "reject")
}

func emitIPv6DNSFilter(b *bpfBuilder, label string, ipOffset uint32, prefix string) {
	portsLabel := prefix + "_ports"
	checkTCPLabel := prefix + "_check_tcp"
	checkDstLabel := prefix + "_check_dst"

	b.label(label)
	b.loadByteAbs(ipOffset + 6)
	b.jumpEqual(ipProtoUDP, portsLabel, checkTCPLabel)
	b.label(checkTCPLabel)
	b.jumpEqual(ipProtoTCP, portsLabel, "reject")

	b.label(portsLabel)
	b.loadHalfAbs(ipOffset + 40)
	b.jumpEqual(53, "accept", checkDstLabel)
	b.label(checkDstLabel)
	b.loadHalfAbs(ipOffset + 42)
	b.jumpEqual(53, "accept", "reject")
}
