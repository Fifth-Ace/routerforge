package main

import "time"

type stringRef uint32

type pooledString struct {
	value string
	refs  uint32
}

type eventStringPool struct {
	byValue map[string]stringRef
	entries []pooledString
	free    []stringRef
}

func newEventStringPool() *eventStringPool {
	return &eventStringPool{
		byValue: make(map[string]stringRef),
		entries: make([]pooledString, 1), // ref 0 is always the empty string
	}
}

func (p *eventStringPool) intern(value string) stringRef {
	if value == "" {
		return 0
	}
	if ref, ok := p.byValue[value]; ok {
		p.entries[ref].refs++
		return ref
	}

	var ref stringRef
	if n := len(p.free); n > 0 {
		ref = p.free[n-1]
		p.free = p.free[:n-1]
		p.entries[ref] = pooledString{value: value, refs: 1}
	} else {
		ref = stringRef(len(p.entries))
		p.entries = append(p.entries, pooledString{value: value, refs: 1})
	}
	p.byValue[value] = ref
	return ref
}

func (p *eventStringPool) release(ref stringRef) {
	if ref == 0 || int(ref) >= len(p.entries) {
		return
	}
	entry := &p.entries[ref]
	if entry.refs == 0 {
		return
	}
	entry.refs--
	if entry.refs != 0 {
		return
	}
	delete(p.byValue, entry.value)
	entry.value = ""
	p.free = append(p.free, ref)
}

func (p *eventStringPool) value(ref stringRef) string {
	if ref == 0 || int(ref) >= len(p.entries) {
		return ""
	}
	return p.entries[ref].value
}

type compactFlowEvent struct {
	Time time.Time

	Profile        stringRef
	Protocol       stringRef
	Upstream       stringRef
	Domain         stringRef
	QType          stringRef
	Transport      stringRef
	ClientIP       stringRef
	ClientMAC      stringRef
	ClientName     stringRef
	ClientHostname stringRef
	ClientPolicy   stringRef
	ClientNetwork  stringRef
	ClientAccess   stringRef
	ClientSSID     stringRef
	ClientAP       stringRef

	Port     uint16
	Fallback bool
}

func compactFlowFromEvent(p *eventStringPool, e FlowEvent) compactFlowEvent {
	return compactFlowEvent{
		Time:           e.Time,
		Profile:        p.intern(e.Profile),
		Protocol:       p.intern(e.Protocol),
		Upstream:       p.intern(e.Upstream),
		Domain:         p.intern(e.Domain),
		QType:          p.intern(e.QType),
		Transport:      p.intern(e.Transport),
		ClientIP:       p.intern(e.ClientIP),
		ClientMAC:      p.intern(e.ClientMAC),
		ClientName:     p.intern(e.ClientName),
		ClientHostname: p.intern(e.ClientHostname),
		ClientPolicy:   p.intern(e.ClientPolicy),
		ClientNetwork:  p.intern(e.ClientNetwork),
		ClientAccess:   p.intern(e.ClientAccess),
		ClientSSID:     p.intern(e.ClientSSID),
		ClientAP:       p.intern(e.ClientAP),
		Port:           e.Port,
		Fallback:       e.Fallback,
	}
}

func (e compactFlowEvent) materialize(p *eventStringPool) FlowEvent {
	return FlowEvent{
		Time:           e.Time,
		Profile:        p.value(e.Profile),
		Protocol:       p.value(e.Protocol),
		Upstream:       p.value(e.Upstream),
		Port:           e.Port,
		Domain:         p.value(e.Domain),
		QType:          p.value(e.QType),
		Transport:      p.value(e.Transport),
		Fallback:       e.Fallback,
		ClientIP:       p.value(e.ClientIP),
		ClientMAC:      p.value(e.ClientMAC),
		ClientName:     p.value(e.ClientName),
		ClientHostname: p.value(e.ClientHostname),
		ClientPolicy:   p.value(e.ClientPolicy),
		ClientNetwork:  p.value(e.ClientNetwork),
		ClientAccess:   p.value(e.ClientAccess),
		ClientSSID:     p.value(e.ClientSSID),
		ClientAP:       p.value(e.ClientAP),
	}
}

func (e compactFlowEvent) release(p *eventStringPool) {
	p.release(e.Profile)
	p.release(e.Protocol)
	p.release(e.Upstream)
	p.release(e.Domain)
	p.release(e.QType)
	p.release(e.Transport)
	p.release(e.ClientIP)
	p.release(e.ClientMAC)
	p.release(e.ClientName)
	p.release(e.ClientHostname)
	p.release(e.ClientPolicy)
	p.release(e.ClientNetwork)
	p.release(e.ClientAccess)
	p.release(e.ClientSSID)
	p.release(e.ClientAP)
}

type compactClientFlowEvent struct {
	Time        time.Time
	CompletedAt time.Time

	ClientIP       stringRef
	ClientMAC      stringRef
	ClientName     stringRef
	ClientHostname stringRef
	ClientPolicy   stringRef
	ClientAccess   stringRef
	Domain         stringRef
	QType          stringRef
	Transport      stringRef
	Outcome        stringRef
	RCode          stringRef
	Resolver       stringRef
	UpstreamRCode  stringRef

	LatencyMS         float64
	UpstreamLatencyMS float64
	ResolverPort      uint16
	Fallback          bool
	UpstreamTimeout   bool
}

func compactClientFlowFromEvent(p *eventStringPool, e ClientFlowEvent) compactClientFlowEvent {
	return compactClientFlowEvent{
		Time:              e.Time,
		CompletedAt:       e.CompletedAt,
		ClientIP:          p.intern(e.ClientIP),
		ClientMAC:         p.intern(e.ClientMAC),
		ClientName:        p.intern(e.ClientName),
		ClientHostname:    p.intern(e.ClientHostname),
		ClientPolicy:      p.intern(e.ClientPolicy),
		ClientAccess:      p.intern(e.ClientAccess),
		Domain:            p.intern(e.Domain),
		QType:             p.intern(e.QType),
		Transport:         p.intern(e.Transport),
		Outcome:           p.intern(e.Outcome),
		RCode:             p.intern(e.RCode),
		Resolver:          p.intern(e.Resolver),
		UpstreamRCode:     p.intern(e.UpstreamRCode),
		LatencyMS:         e.LatencyMS,
		ResolverPort:      e.ResolverPort,
		Fallback:          e.Fallback,
		UpstreamLatencyMS: e.UpstreamLatencyMS,
		UpstreamTimeout:   e.UpstreamTimeout,
	}
}

func (e compactClientFlowEvent) materialize(p *eventStringPool) ClientFlowEvent {
	return ClientFlowEvent{
		Time:              e.Time,
		CompletedAt:       e.CompletedAt,
		ClientIP:          p.value(e.ClientIP),
		ClientMAC:         p.value(e.ClientMAC),
		ClientName:        p.value(e.ClientName),
		ClientHostname:    p.value(e.ClientHostname),
		ClientPolicy:      p.value(e.ClientPolicy),
		ClientAccess:      p.value(e.ClientAccess),
		Domain:            p.value(e.Domain),
		QType:             p.value(e.QType),
		Transport:         p.value(e.Transport),
		Outcome:           p.value(e.Outcome),
		RCode:             p.value(e.RCode),
		LatencyMS:         e.LatencyMS,
		Resolver:          p.value(e.Resolver),
		ResolverPort:      e.ResolverPort,
		Fallback:          e.Fallback,
		UpstreamRCode:     p.value(e.UpstreamRCode),
		UpstreamLatencyMS: e.UpstreamLatencyMS,
		UpstreamTimeout:   e.UpstreamTimeout,
	}
}

func (e compactClientFlowEvent) release(p *eventStringPool) {
	p.release(e.ClientIP)
	p.release(e.ClientMAC)
	p.release(e.ClientName)
	p.release(e.ClientHostname)
	p.release(e.ClientPolicy)
	p.release(e.ClientAccess)
	p.release(e.Domain)
	p.release(e.QType)
	p.release(e.Transport)
	p.release(e.Outcome)
	p.release(e.RCode)
	p.release(e.Resolver)
	p.release(e.UpstreamRCode)
}
