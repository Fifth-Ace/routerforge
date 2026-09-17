package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
	"github.com/Fifth-Ace/routerforge/internal/platform/transaction"
)

const platformEventProducerResponseLimit int64 = 128 << 10

type moduleHealthObservation struct {
	Healthy   bool
	ChangedAt time.Time
	Detail    string
}

var moduleHealthProducerState = struct {
	sync.Mutex
	Modules map[string]moduleHealthObservation
}{
	Modules: make(map[string]moduleHealthObservation),
}

type adminServiceActionEnvelope struct {
	OK          bool                 `json:"ok"`
	Error       string               `json:"error"`
	Action      string               `json:"action"`
	ID          string               `json:"id"`
	Transaction transaction.Manifest `json:"transaction"`
}

func observeModuleProxyResponse(moduleID, targetPath, method string, resp *http.Response) {
	if resp == nil {
		return
	}
	if targetPath == "/v1/health" {
		observeModuleHealth(moduleID, resp.StatusCode >= 200 && resp.StatusCode < 300, resp.Status)
	}
	if moduleID == "admin" && method == http.MethodPost && serviceActionTarget(targetPath) {
		observeAdminServiceActionResponse(targetPath, resp)
	}
}

func observeModuleProxyError(moduleID, targetPath string, err error) {
	if targetPath != "/v1/health" {
		return
	}
	detail := "module transport unavailable"
	if err != nil {
		detail = err.Error()
	}
	observeModuleHealth(moduleID, false, detail)
}

func observeModuleHealth(moduleID string, healthy bool, detail string) {
	moduleID = strings.ToLower(strings.TrimSpace(moduleID))
	if moduleID == "" {
		return
	}

	moduleHealthProducerState.Lock()
	previous, seen := moduleHealthProducerState.Modules[moduleID]
	if seen && previous.Healthy == healthy {
		moduleHealthProducerState.Unlock()
		return
	}
	now := time.Now().UTC()
	moduleHealthProducerState.Modules[moduleID] = moduleHealthObservation{
		Healthy:   healthy,
		ChangedAt: now,
		Detail:    boundedEventText(detail, 512),
	}
	moduleHealthProducerState.Unlock()

	event := platformevents.Event{
		Component: moduleID,
		ObjectID:  moduleID,
		Context: map[string]any{
			"producer": "module-proxy-health",
			"detail":   boundedEventText(detail, 512),
		},
	}

	if healthy {
		event.Severity = platformevents.Info
		event.Type = "module.health.up"
		event.Message = "module health is available"
		if seen {
			event.Type = "module.health.recovered"
			event.Message = "module health recovered"
			event.Recovery = true
		}
	} else {
		event.Severity = platformevents.Warning
		event.Type = "module.health.down"
		event.Message = "module health is unavailable"
	}

	emitPlatformEvent(event)
}

const healthAlertCriticalAfter = 5 * time.Minute

func snapshotActiveModuleHealthAlerts(now time.Time) []healthAlert {
	moduleHealthProducerState.Lock()
	snapshot := make(map[string]moduleHealthObservation, len(moduleHealthProducerState.Modules))
	for moduleID, observation := range moduleHealthProducerState.Modules {
		snapshot[moduleID] = observation
	}
	moduleHealthProducerState.Unlock()

	alerts := make([]healthAlert, 0, len(snapshot))
	for moduleID, observation := range snapshot {
		if observation.Healthy {
			continue
		}
		since := observation.ChangedAt
		if since.IsZero() {
			since = now
		}
		age := now.Sub(since)
		if age < 0 {
			age = 0
		}
		severity := platformevents.Warning
		if age >= healthAlertCriticalAfter {
			severity = platformevents.Critical
		}
		context := map[string]any{
			"producer": "module-proxy-health",
		}
		if observation.Detail != "" {
			context["detail"] = observation.Detail
		}
		alerts = append(alerts, healthAlert{
			ID:         "module-health:" + moduleID,
			Component:  moduleID,
			Severity:   severity,
			State:      "active",
			Message:    "module health is unavailable",
			Since:      since,
			AgeSeconds: int64(age / time.Second),
			Context:    context,
		})
	}

	sort.Slice(alerts, func(i, j int) bool {
		if alerts[i].Severity != alerts[j].Severity {
			return alerts[i].Severity == platformevents.Critical
		}
		if !alerts[i].Since.Equal(alerts[j].Since) {
			return alerts[i].Since.Before(alerts[j].Since)
		}
		return alerts[i].Component < alerts[j].Component
	})
	return alerts
}

func serviceActionTarget(targetPath string) bool {
	if !strings.HasPrefix(targetPath, "/v1/services/") || !strings.HasSuffix(targetPath, "/action") {
		return false
	}
	serviceID := strings.TrimSuffix(strings.TrimPrefix(targetPath, "/v1/services/"), "/action")
	return serviceID != "" && !strings.Contains(serviceID, "/")
}

func serviceIDFromActionTarget(targetPath string) string {
	if !serviceActionTarget(targetPath) {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(targetPath, "/v1/services/"), "/action")
}

func observeAdminServiceActionResponse(targetPath string, resp *http.Response) {
	body, ok := snapshotProducerResponseBody(resp)
	if !ok {
		return
	}

	var envelope adminServiceActionEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return
	}

	serviceID := strings.TrimSpace(envelope.ID)
	if serviceID == "" {
		serviceID = serviceIDFromActionTarget(targetPath)
	}
	if serviceID == "" {
		return
	}

	action := strings.ToLower(strings.TrimSpace(envelope.Action))
	txState := string(envelope.Transaction.State)
	context := map[string]any{
		"producer":    "admin-service-proxy",
		"http_status": resp.StatusCode,
	}
	if action != "" {
		context["action"] = action
	}
	if txState != "" {
		context["transaction_state"] = txState
	}
	if strings.TrimSpace(envelope.Error) != "" {
		context["error"] = boundedEventText(envelope.Error, 512)
	}

	serviceEvent := platformevents.Event{
		Component:     "admin-service",
		ObjectID:      serviceID,
		TransactionID: envelope.Transaction.ID,
		Severity:      platformevents.Warning,
		Type:          "service.action.rejected",
		Message:       "service action was not committed",
		Context:       context,
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 && envelope.Transaction.State == transaction.Committed {
		serviceEvent.Severity = platformevents.Info
		serviceEvent.Type = "service.action.committed"
		serviceEvent.Message = "service action committed"
	} else if envelope.Transaction.ID != "" {
		serviceEvent.Type = "service.action.failed"
		serviceEvent.Message = "service action transaction failed"
	}

	emitPlatformEvent(serviceEvent)

	if envelope.Transaction.ID == "" {
		return
	}

	txSeverity := platformevents.Info
	switch envelope.Transaction.State {
	case transaction.Failed, transaction.Ambiguous:
		txSeverity = platformevents.Warning
	case transaction.RolledBack:
		txSeverity = platformevents.Warning
	}

	txEvent := platformevents.Event{
		Component:     strings.TrimSpace(envelope.Transaction.Component),
		ObjectID:      serviceID,
		TransactionID: envelope.Transaction.ID,
		Severity:      txSeverity,
		Type:          "transaction." + string(envelope.Transaction.State),
		Message:       "transaction state observed",
		Context: map[string]any{
			"producer":    "admin-service-proxy",
			"action":      action,
			"http_status": resp.StatusCode,
		},
	}
	if txEvent.Component == "" {
		txEvent.Component = "admin-service"
	}
	if envelope.Transaction.State == transaction.RolledBack {
		txEvent.Recovery = true
	}
	emitPlatformEvent(txEvent)
}

func snapshotProducerResponseBody(resp *http.Response) ([]byte, bool) {
	if resp == nil || resp.Body == nil || resp.Body == http.NoBody {
		return nil, false
	}
	if resp.ContentLength > platformEventProducerResponseLimit {
		return nil, false
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, platformEventProducerResponseLimit+1))
	if err != nil {
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), resp.Body))
		return nil, false
	}
	if int64(len(body)) > platformEventProducerResponseLimit {
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), resp.Body))
		return nil, false
	}

	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return body, true
}

func boundedEventText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
