package main

// strictKnockModel describes the desired fail-closed sequence, not the
// current iptables rules. This is a pure simulation: no firewall calls.
type strictKnockModel struct {
	states map[string]strictKnockState
}

type strictKnockState struct {
	stage   int
	started int
	last    int
	granted int
}

func newStrictKnockModel() *strictKnockModel {
	return &strictKnockModel{states: make(map[string]strictKnockState)}
}

// packet returns whether the protected target is authorized at the given second.
// Out-of-order packets on knock ports reset progress; step 1 restarts the sequence.
func (m *strictKnockModel) packet(src string, port, now, window, ttl, target int) bool {
	s := m.states[src]
	if s.stage != 0 && s.stage != 3 && now-s.started > window {
		s = strictKnockState{}
	}
	if s.stage == 3 && now-s.granted > ttl {
		s = strictKnockState{}
	}
	switch port {
	case 41001:
		s = strictKnockState{stage: 1, started: now, last: now}
	case 41002:
		if s.stage == 1 && now-s.last <= window {
			s.stage = 2
			s.last = now
		} else {
			s = strictKnockState{}
		}
	case 41003:
		if s.stage == 2 && now-s.last <= window {
			s.stage = 3
			s.granted = now
		} else {
			s = strictKnockState{}
		}
	default:
		m.states[src] = s
		return port == target && s.stage == 3 && now-s.granted <= ttl
	}
	m.states[src] = s
	return false
}
