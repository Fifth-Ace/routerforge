package transaction

import (
	"errors"
	"fmt"
	"strings"
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

type EvidenceStatus string

const (
	EvidenceInfo      EvidenceStatus = "info"
	EvidencePassed    EvidenceStatus = "passed"
	EvidenceFailed    EvidenceStatus = "failed"
	EvidenceRecovered EvidenceStatus = "recovered"
)

const MaxEvidenceEntries = 64

type Evidence struct {
	Time    time.Time         `json:"time"`
	Stage   string            `json:"stage"`
	Status  EvidenceStatus    `json:"status"`
	Message string            `json:"message,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

type Manifest struct {
	ID        string     `json:"id"`
	Component string     `json:"component"`
	State     State      `json:"state"`
	StartedAt time.Time  `json:"started_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Artifacts []string   `json:"artifacts,omitempty"`
	Evidence  []Evidence `json:"evidence,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type Error struct {
	Err      error
	Manifest Manifest
}

func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return "transaction failed"
	}
	return e.Err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(id, component string, artifacts ...string) Manifest {
	now := time.Now().UTC()
	id = strings.TrimSpace(id)
	if id == "" {
		id = fmt.Sprintf("tx-%d", now.UnixNano())
	}
	manifest := Manifest{
		ID:        id,
		Component: strings.TrimSpace(component),
		State:     Precheck,
		StartedAt: now,
		UpdatedAt: now,
		Artifacts: append([]string(nil), artifacts...),
	}
	Record(&manifest, string(Precheck), EvidenceInfo, "transaction started", nil)
	return manifest
}

func cloneDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]string, len(details))
	for key, value := range details {
		out[key] = value
	}
	return out
}

func Clone(manifest Manifest) Manifest {
	out := manifest
	out.Artifacts = append([]string(nil), manifest.Artifacts...)
	if len(manifest.Evidence) != 0 {
		out.Evidence = make([]Evidence, len(manifest.Evidence))
		for i, entry := range manifest.Evidence {
			out.Evidence[i] = entry
			out.Evidence[i].Details = cloneDetails(entry.Details)
		}
	}
	return out
}

func Record(manifest *Manifest, stage string, status EvidenceStatus, message string, details map[string]string) {
	if manifest == nil {
		return
	}
	now := time.Now().UTC()
	entry := Evidence{
		Time:    now,
		Stage:   strings.TrimSpace(stage),
		Status:  status,
		Message: strings.TrimSpace(message),
		Details: cloneDetails(details),
	}
	if len(manifest.Evidence) >= MaxEvidenceEntries {
		// Keep the first event (transaction origin) and the newest evidence.
		manifest.Evidence = append(manifest.Evidence[:1], manifest.Evidence[2:]...)
	}
	manifest.Evidence = append(manifest.Evidence, entry)
	manifest.UpdatedAt = now
}

func RecordFailure(manifest *Manifest, stage string, err error, details map[string]string) {
	if manifest == nil || err == nil {
		return
	}
	manifest.Error = err.Error()
	Record(manifest, stage, EvidenceFailed, err.Error(), details)
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

func transitionEvidence(to State) (EvidenceStatus, string) {
	switch to {
	case RolledBack:
		return EvidenceRecovered, "transaction rolled back"
	case Ambiguous:
		return EvidenceFailed, "transaction state is ambiguous"
	case Failed:
		return EvidenceFailed, "transaction failed"
	default:
		return EvidencePassed, "transaction entered " + string(to)
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
	status, message := transitionEvidence(to)
	Record(manifest, string(to), status, message, nil)
	return nil
}

func Wrap(err error, manifest Manifest) error {
	if err == nil {
		return nil
	}
	return &Error{Err: err, Manifest: Clone(manifest)}
}

func Extract(err error) (Manifest, bool) {
	var txErr *Error
	if !errors.As(err, &txErr) || txErr == nil {
		return Manifest{}, false
	}
	return Clone(txErr.Manifest), true
}
