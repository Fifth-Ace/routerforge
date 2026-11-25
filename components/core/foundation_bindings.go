package main

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	platformevents "github.com/Fifth-Ace/routerforge/internal/platform/events"
)

// A single lock serializes catalogue saves and binding operations. The native
// module's own CAS is still authoritative for concurrent native-editor changes.
type foundationBindingService struct {
	mu    sync.Mutex
	store *contract.Store
	path  string
	read  foundationNativeRead
	call  bindingCall
}
type bindingRequest struct {
	Operation     string `json:"operation"`
	Target        string `json:"target"`
	GroupID       string `json:"groupId"`
	CatalogueETag string `json:"catalogueETag"`
	Token         string `json:"token,omitempty"`
}
type bindingPlan struct {
	Operation      string                    `json:"operation"`
	Target         string                    `json:"target"`
	GroupID        string                    `json:"groupId"`
	CatalogueETag  string                    `json:"catalogueETag"`
	Generation     int64                     `json:"generation"`
	Status         string                    `json:"status"`
	CanCommit      bool                      `json:"canCommit"`
	Before         *contract.BindingSnapshot `json:"before,omitempty"`
	After          *contract.BindingSnapshot `json:"after,omitempty"`
	NativeRevision string                    `json:"nativeRevision,omitempty"`
	Added          []string                  `json:"added"`
	Removed        []string                  `json:"removed"`
	Token          string                    `json:"token"`
	Runtime        string                    `json:"runtime"`
}

func (s *foundationBindingService) guard(r contract.Revision) error {
	st, e := contract.ReadBindingState(s.path)
	if e != nil {
		return e
	}
	return contract.CheckBindingCatalog(r, st)
}
func bindingIndex(st contract.BindingState, target string) int {
	for i, b := range st.Bindings {
		if b.Target == target {
			return i
		}
	}
	return -1
}
func (s *foundationBindingService) persist(st *contract.BindingState) error {
	st.Generation++
	return contract.WriteBindingState(s.path, *st)
}
func bindingDiff(before, after []string) (added, removed []string) {
	added = []string{}
	removed = []string{}
	a, b := map[string]bool{}, map[string]bool{}
	for _, v := range before {
		a[v] = true
	}
	for _, v := range after {
		b[v] = true
		if !a[v] {
			added = append(added, v)
		}
	}
	for _, v := range before {
		if !b[v] {
			removed = append(removed, v)
		}
	}
	return
}
func (s *foundationBindingService) plan(ctx context.Context, cat contract.Revision, st contract.BindingState, q bindingRequest) (bindingPlan, bindingWrite, error) {
	p := bindingPlan{Operation: q.Operation, Target: q.Target, GroupID: q.GroupID, CatalogueETag: cat.ETag, Generation: st.Generation, Status: "blocked", Added: []string{}, Removed: []string{}, Runtime: "NOT_VERIFIED"}
	var write bindingWrite
	ref, ok := contract.BindingTarget(q.Target)
	if !ok || q.CatalogueETag != cat.ETag {
		return p, write, contract.ErrConflict
	}
	i := bindingIndex(st, q.Target)
	var b contract.Binding
	if i >= 0 {
		b = st.Bindings[i]
		if q.GroupID != b.GroupID {
			return p, write, contract.ErrConflict
		}
	}
	switch q.Operation {
	case "attach":
		if i >= 0 || len(st.Bindings) >= contract.MaxBindings {
			return p, write, contract.ErrBinding
		}
	case "inspect", "sync", "rollback", "rebase", "reconcile", "detach":
		if i < 0 {
			return p, write, contract.ErrBinding
		}
	default:
		return p, write, contract.ErrBinding
	}
	// Detach removes metadata only (including an unresolved journal if expressly
	// chosen). It remains possible when the native module is absent.
	if q.Operation == "detach" {
		p.Before = &b.Checkpoint
		p.Status = "detach_metadata_only"
		p.CanCommit = true
		p.Token = contract.BindingDigest(p)
		return p, write, nil
	}
	target, e := bindingReadTarget(ctx, s.call, ref)
	if e != nil {
		return p, write, e
	}
	p.Before = &target.Snapshot
	p.NativeRevision = target.Revision
	if q.Operation == "inspect" && ref.Module == "dns" {
		p.Runtime = bindingDNSRuntime(ctx, s.call, target.Revision)
	}
	if b.Pending != nil {
		p.After = &b.Pending.After
		switch {
		case reflect.DeepEqual(target.Snapshot, b.Pending.After):
			p.Status = "pending_after_observed"
		case reflect.DeepEqual(target.Snapshot, b.Pending.Before):
			p.Status = "pending_before_observed"
		default:
			p.Status = "pending_conflict"
		}
		p.CanCommit = q.Operation == "reconcile" && p.Status != "pending_conflict"
		p.Token = contract.BindingDigest(p)
		return p, write, nil
	}
	if q.Operation == "reconcile" {
		return p, write, contract.ErrBinding
	}
	if q.Operation == "attach" {
		check := st
		check.Bindings = append(append([]contract.Binding{}, st.Bindings...), contract.Binding{Target: q.Target, GroupID: q.GroupID, Checkpoint: target.Snapshot})
		if e = contract.CheckBindingCatalog(cat, check); e != nil {
			return p, write, e
		}
	} else if e = contract.CheckBindingCatalog(cat, st); e != nil {
		return p, write, e
	}
	if q.Operation == "rebase" {
		p.After = &target.Snapshot
		p.Status = "rebase_metadata_only"
		p.CanCommit = true
		p.Token = contract.BindingDigest(p)
		return p, write, nil
	}
	if i >= 0 && !reflect.DeepEqual(target.Snapshot, b.Checkpoint) {
		p.Status = "native_conflict"
		p.Token = contract.BindingDigest(p)
		return p, write, nil
	}
	var values []string
	if q.Operation == "rollback" {
		if b.Previous == nil {
			return p, write, contract.ErrBinding
		}
		values = b.Previous.Values
	} else {
		consumer := "dns"
		if ref.Module == "nfqws" {
			consumer = "nfqws-domain"
		}
		projection, e := contract.ProjectConsumerWithNative(cat, consumer, q.GroupID, foundationNativeResolver(ctx, s.read))
		if e != nil || len(projection.Blocked) != 0 {
			return p, write, contract.ErrBinding
		}
		values, e = contract.BindingDomains(projection.Values)
		if e != nil {
			return p, write, e
		}
	}
	p.Added, p.Removed = bindingDiff(target.Snapshot.Values, values)
	if q.Operation == "attach" || q.Operation == "inspect" {
		p.After = &contract.BindingSnapshot{Values: values}
		p.Status = "needs_sync"
		if reflect.DeepEqual(values, target.Snapshot.Values) {
			p.Status = "saved_in_sync"
		}
		p.CanCommit = q.Operation == "attach"
		p.Token = contract.BindingDigest(p)
		return p, write, nil
	}
	after, write, e := bindingPrepare(ctx, s.call, ref, target, values)
	if e != nil {
		return p, write, e
	}
	p.After = &after
	if q.Operation == "rollback" && !reflect.DeepEqual(after, *b.Previous) {
		return p, write, contract.ErrConflict
	}
	p.Status = "ready"
	p.CanCommit = write.Method != ""
	if !p.CanCommit {
		p.Status = "saved_in_sync"
	}
	p.Token = contract.BindingDigest(p)
	return p, write, nil
}
func bindingEvent(kind string, severity platformevents.Severity) {
	emitPlatformEvent(platformevents.Event{Component: "policy", Type: "policy.binding." + kind, Severity: severity, Message: "Policy binding " + kind})
}
func (s *foundationBindingService) finish(st *contract.BindingState, i int, after bool) error {
	b := &st.Bindings[i]
	p := b.Pending
	if p == nil {
		return contract.ErrBinding
	}
	if after {
		old := p.Before
		b.Checkpoint = p.After
		b.Previous = &old
	}
	b.Pending = nil
	if e := s.persist(st); e != nil {
		return e
	}
	if after {
		bindingEvent(p.Operation+".saved", platformevents.Info)
	} else {
		bindingEvent("reconciled.before", platformevents.Warning)
	}
	return nil
}
func (s *foundationBindingService) commit(ctx context.Context, cat contract.Revision, st *contract.BindingState, q bindingRequest) (bindingPlan, error) {
	p, write, e := s.plan(ctx, cat, *st, q)
	if e != nil {
		return p, e
	}
	if !p.CanCommit || q.Token == "" || p.Token != q.Token {
		return p, contract.ErrConflict
	}
	i := bindingIndex(*st, q.Target)
	switch q.Operation {
	case "attach":
		st.Bindings = append(st.Bindings, contract.Binding{Target: q.Target, GroupID: q.GroupID, Checkpoint: *p.Before})
		if e = s.persist(st); e == nil {
			bindingEvent("attached", platformevents.Info)
		}
	case "detach":
		st.Bindings = append(st.Bindings[:i], st.Bindings[i+1:]...)
		if e = s.persist(st); e == nil {
			bindingEvent("detached", platformevents.Info)
		}
	case "rebase":
		b := &st.Bindings[i]
		b.Checkpoint = *p.Before
		b.Previous = nil
		if e = s.persist(st); e == nil {
			bindingEvent("rebased", platformevents.Info)
		}
	case "reconcile":
		e = s.finish(st, i, p.Status == "pending_after_observed")
	case "sync", "rollback":
		st.Bindings[i].Pending = &contract.BindingPending{Operation: q.Operation, Before: *p.Before, After: *p.After}
		if e = s.persist(st); e != nil {
			return p, e
		}
		// If the write, reply, readback or final journal update fails, retain the
		// durable pending record. Never retry a native mutation or guess rollback.
		bindingEvent("pending", platformevents.Info)
		if _, e = s.call(ctx, write.Module, write.Method, write.Path, write.Headers, write.Body); e != nil {
			bindingEvent("uncertain", platformevents.Warning)
			return p, contract.ErrConflict
		}
		ref, _ := contract.BindingTarget(q.Target)
		observed, e := bindingReadTarget(ctx, s.call, ref)
		if e != nil || !reflect.DeepEqual(observed.Snapshot, *p.After) {
			bindingEvent("uncertain", platformevents.Warning)
			return p, contract.ErrConflict
		}
		// persist sorts the registry; recover the index by target after journal write.
		e = s.finish(st, bindingIndex(*st, q.Target), true)
	default:
		return p, contract.ErrBinding
	}
	if e != nil {
		return p, e
	}
	return p, nil
}

// Authorize with the same root-session policy as the native module mutation proxy.
func registerFoundationBindings(mux *http.ServeMux, s *foundationBindingService, allowed func(*http.Request) bool) {
	mux.HandleFunc(foundationPath+"/bindings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		if r.URL.RawQuery != "" {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		st, e := contract.ReadBindingState(s.path)
		if e != nil {
			foundationError(w, e)
			return
		}
		cat, e := s.store.Get()
		if e != nil {
			foundationError(w, e)
			return
		}
		rows := []map[string]any{}
		for _, b := range st.Bindings {
			rows = append(rows, map[string]any{"target": b.Target, "groupId": b.GroupID, "sha256": b.Checkpoint.SHA256, "count": len(b.Checkpoint.Values), "rollbackAvailable": b.Previous != nil, "pending": b.Pending != nil, "runtime": "NOT_VERIFIED"})
		}
		writeAuthJSON(w, 200, map[string]any{"schemaVersion": 1, "generation": st.Generation, "catalogueETag": cat.Current.ETag, "bindings": rows})
	})
	action := func(commit bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				methodNotAllowedJSON(w, http.MethodPost)
				return
			}
			if !sameOriginRequest(r) || r.Header.Get("X-RF-Foundation") != "1" || !allowed(r) {
				writeAuthJSON(w, 403, map[string]string{"error": "BINDING_AUTH_REQUIRED"})
				return
			}
			if commit && r.Header.Get("X-RF-Confirm") != "BINDING_COMMIT" {
				writeAuthJSON(w, 400, map[string]string{"error": "CONFIRM_REQUIRED"})
				return
			}
			media, _, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if e != nil || media != "application/json" {
				writeAuthJSON(w, 415, map[string]string{"error": "JSON_REQUIRED"})
				return
			}
			raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 8192))
			if e != nil {
				writeAuthJSON(w, 400, map[string]string{"error": "INVALID_REQUEST"})
				return
			}
			if _, e = contract.Decode(raw); e != nil {
				writeAuthJSON(w, 400, map[string]string{"error": "INVALID_REQUEST"})
				return
			}
			var q bindingRequest
			obj, e := bindingObject(raw)
			if e != nil || json.Unmarshal(raw, &q) != nil {
				writeAuthJSON(w, 400, map[string]string{"error": "INVALID_REQUEST"})
				return
			}
			for k := range obj {
				if k != "operation" && k != "target" && k != "groupId" && k != "catalogueETag" && k != "token" {
					writeAuthJSON(w, 400, map[string]string{"error": "INVALID_REQUEST"})
					return
				}
			}
			if len(q.GroupID) > 96 || len(q.Target) > 96 || len(q.Token) > 64 || len(q.Operation) > 16 || len(q.CatalogueETag) != 64 {
				writeAuthJSON(w, 400, map[string]string{"error": "INVALID_REQUEST"})
				return
			}
			s.mu.Lock()
			defer s.mu.Unlock()
			ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
			defer cancel()
			st, e := contract.ReadBindingState(s.path)
			if e != nil {
				foundationError(w, e)
				return
			}
			cat, e := s.store.Get()
			if e != nil {
				foundationError(w, e)
				return
			}
			var p bindingPlan
			if commit {
				p, e = s.commit(ctx, cat.Current, &st, q)
			} else {
				p, _, e = s.plan(ctx, cat.Current, st, q)
			}
			if e != nil {
				if e == contract.ErrStorage {
					foundationError(w, e)
				} else {
					writeAuthJSON(w, 409, map[string]string{"error": "BINDING_CONFLICT_OR_UNSUPPORTED_RELOAD_AND_INSPECT"})
				}
				return
			}
			if commit {
				writeAuthJSON(w, 200, map[string]any{"schemaVersion": 1, "committed": true, "generation": st.Generation, "runtime": "NOT_VERIFIED"})
				return
			}
			writeAuthJSON(w, 200, p)
		}
	}
	mux.HandleFunc(foundationPath+"/bindings/preview", action(false))
	mux.HandleFunc(foundationPath+"/bindings/commit", action(true))
}
