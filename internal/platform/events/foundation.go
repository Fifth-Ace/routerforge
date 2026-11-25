package events

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// NewEventID returns an opaque UUIDv4. If entropy is unavailable, callers do not
// export an unidentifiable event; the existing legacy ring is still functional.
func NewEventID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// WireEvent deliberately excludes raw messages, context, paths and transaction
// payloads from existing producers. No claims about a stable device mapping yet.
type WireEvent struct {
	SchemaVersion int       `json:"schemaVersion"`
	ID            string    `json:"id"`
	ObservedAt    time.Time `json:"observedAt"`
	ObserverID    string    `json:"observerId"`
	Component     string    `json:"component"`
	Severity      Severity  `json:"severity"`
	Type          string    `json:"type"`
	MessageCode   string    `json:"messageCode"`
	DeviceID      *string   `json:"deviceId"`
	ObjectID      *string   `json:"objectId"`
	CorrelationID *string   `json:"correlationId"`
	Recovery      bool      `json:"recovery"`
}

var wireTypes = map[string]string{
	"core.ready": "core", "device.network.down": "network", "device.network.missing": "network", "device.network.recovered": "network", "device.storage.warning": "storage", "device.storage.critical": "storage", "device.storage.recovered": "storage", "device.thermal.warning": "thermal", "device.thermal.critical": "thermal", "device.thermal.recovered": "thermal", "service.action.rejected": "admin-service", "service.action.failed": "admin-service", "service.action.committed": "admin-service", "policy.catalog.saved": "policy",
}
var wireModules = map[string]bool{"core": true, "dns": true, "dns-router": true, "dns-router-lite": true, "admin": true, "monitoring": true, "network-tools": true, "nfqws-manager": true, "profiling": true}

func Normalize(e Event, observerID string) (WireEvent, bool) {
	if e.ID == "" || observerID == "" || e.Time.IsZero() || (e.Severity != Info && e.Severity != Warning && e.Severity != Critical) {
		return WireEvent{}, false
	}
	component, ok := wireTypes[e.Type]
	if !ok {
		switch e.Type {
		case "module.health.up", "module.health.down", "module.health.recovered":
			if wireModules[e.Component] {
				component = e.Component
				ok = true
			}
		}
	}
	if !ok || component != e.Component {
		return WireEvent{}, false
	}
	return WireEvent{SchemaVersion: 1, ID: e.ID, ObservedAt: e.Time.UTC(), ObserverID: observerID, Component: component, Severity: e.Severity, Type: e.Type, MessageCode: strings.ToUpper(strings.ReplaceAll(e.Type, ".", "_")), Recovery: e.Recovery}, true
}
