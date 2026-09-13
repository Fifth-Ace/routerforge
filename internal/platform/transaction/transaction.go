package transaction

import (
	"fmt"
	"time"
)

type State string

const (
	Precheck   State = "precheck"
	Snapshot   State = "snapshot"
	Validated  State = "validated"
	Applied    State = "applied"
	Verified   State = "verified"
	Committed  State = "committed"
	RolledBack State = "rolled-back"
	Ambiguous  State = "ambiguous"
	Failed     State = "failed"
)

type Manifest struct {
	ID        string    `json:"id"`
	Component string    `json:"component"`
	State     State     `json:"state"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Artifacts []string  `json:"artifacts,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func CanTransition(from, to State) bool {
	switch from {
	case Precheck:
		return to == Snapshot || to == Failed
	case Snapshot:
		return to == Validated || to == Failed
	case Validated:
		return to == Applied || to == Failed
	case Applied:
		return to == Verified || to == RolledBack || to == Ambiguous || to == Failed
	case Verified:
		return to == Committed || to == RolledBack || to == Ambiguous
	case RolledBack, Committed, Ambiguous, Failed:
		return false
	default:
		return false
	}
}

func Advance(manifest *Manifest, to State) error {
	if manifest == nil {
		return fmt.Errorf("transaction manifest is nil")
	}
	if !CanTransition(manifest.State, to) {
		return fmt.Errorf("invalid transaction transition %s -> %s", manifest.State, to)
	}
	manifest.State = to
	manifest.UpdatedAt = time.Now()
	return nil
}
