package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

// Binding state is separate from the portable catalogue: target names and
// recovery membership are installation-local, never credentials/config blobs.
const MaxBindings = 16
const MaxBindingBytes = 2 << 20

var ErrBinding = errors.New("invalid binding or dependency")
var bindingHash = regexp.MustCompile(`^[a-f0-9]{64}$`)

type BindingSnapshot struct {
	SHA256 string   `json:"sha256"`
	Values []string `json:"values"`
}
type BindingPending struct {
	Operation string          `json:"operation"`
	Before    BindingSnapshot `json:"before"`
	After     BindingSnapshot `json:"after"`
}
type Binding struct {
	Target     string           `json:"target"`
	GroupID    string           `json:"groupId"`
	Checkpoint BindingSnapshot  `json:"checkpoint"`
	Previous   *BindingSnapshot `json:"previous,omitempty"`
	Pending    *BindingPending  `json:"pending,omitempty"`
}
type BindingState struct {
	SchemaVersion int       `json:"schemaVersion"`
	Generation    int64     `json:"generation"`
	Bindings      []Binding `json:"bindings"`
}

// Dedicated pre-existing lists only. No protected/default/auto lists, paths,
// unqualified references or source pins can become write targets.
func BindingTarget(key string) (policy.NativeListReference, bool) {
	ref, ok := policy.ParseNativeListReference(key)
	if !ok || ref.SHA256 != "" || !strings.HasPrefix(ref.Name, "rf-binding-") {
		return ref, false
	}
	tail := strings.TrimPrefix(ref.Name, "rf-binding-")
	if ref.Module == "nfqws" {
		tail = strings.TrimSuffix(tail, ".list")
	}
	return ref, tail != ""
}
func BindingDomains(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > NativeListLimit {
		return nil, ErrBinding
	}
	out, e := nativeDomains(values)
	if e != nil || len(out) == 0 {
		return nil, ErrBinding
	}
	return out, nil
}
func BindingDigest(v any) string               { b, _ := json.Marshal(v); return nativeDigest(b) }
func BindingText(values []string) string       { return strings.Join(values, "\n") + "\n" }
func BindingTextDigest(values []string) string { return nativeDigest([]byte(BindingText(values))) }
func validBindingSnapshot(s BindingSnapshot) bool {
	v, e := BindingDomains(s.Values)
	return e == nil && bindingHash.MatchString(s.SHA256) && reflect.DeepEqual(v, s.Values)
}
func ValidateBindingState(st BindingState) error {
	if st.SchemaVersion != 1 || st.Generation < 0 || st.Generation >= 9007199254740991 || st.Bindings == nil || len(st.Bindings) > MaxBindings {
		return ErrBinding
	}
	seen := map[string]bool{}
	for _, b := range st.Bindings {
		ref, ok := BindingTarget(b.Target)
		if !ok || seen[b.Target] || b.GroupID == "" || len(b.GroupID) > 96 || !validBindingSnapshot(b.Checkpoint) {
			return ErrBinding
		}
		seen[b.Target] = true
		if b.Previous != nil && !validBindingSnapshot(*b.Previous) {
			return ErrBinding
		}
		if p := b.Pending; p != nil {
			if (p.Operation != "sync" && p.Operation != "rollback") || !validBindingSnapshot(p.Before) || !validBindingSnapshot(p.After) || !reflect.DeepEqual(p.Before, b.Checkpoint) || p.Before.SHA256 == p.After.SHA256 {
				return ErrBinding
			}
			if p.Operation == "rollback" && (b.Previous == nil || !reflect.DeepEqual(*b.Previous, p.After)) {
				return ErrBinding
			}
		}
		if ref.Module == "nfqws" {
			for _, s := range []*BindingSnapshot{&b.Checkpoint, b.Previous} {
				if s != nil && BindingTextDigest(s.Values) != s.SHA256 {
					return ErrBinding
				}
			}
			if b.Pending != nil && BindingTextDigest(b.Pending.After.Values) != b.Pending.After.SHA256 {
				return ErrBinding
			}
		}
	}
	return nil
}

// All bound groups must remain present and domain-typed. Forbid using ANY
// bound destination as a source in a bound group (including nested groups).
// This conservatively prevents self-dependencies and cross-binding cycles.
func CheckBindingCatalog(r Revision, st BindingState) error {
	groups, _, e := consumerCatalog(r)
	if e != nil {
		return e
	}
	targets := map[string]bool{}
	byID := map[string]policy.Group{}
	for _, b := range st.Bindings {
		targets[b.Target] = true
	}
	for _, g := range groups {
		byID[g.ID] = g
	}
	for _, b := range st.Bindings {
		g, ok := byID[b.GroupID]
		if !ok || g.Type != policy.DomainGroup {
			return ErrBinding
		}
		leaves, e := policy.Expand(groups, g.ID)
		if e != nil {
			return e
		}
		for _, m := range leaves {
			if m.Kind == "list-ref" {
				ref, ok := policy.ParseNativeListReference(m.Value)
				if ok && targets[ref.Key()] {
					return ErrBinding
				}
			}
		}
	}
	return nil
}

// Caller serializes catalogue saves and all binding operations in Core. No
// startup writes, no recovery by guessing after malformed state.
func ReadBindingState(path string) (BindingState, error) {
	empty := BindingState{1, 0, []Binding{}}
	info, e := os.Lstat(path)
	if os.IsNotExist(e) {
		return empty, nil
	}
	if e != nil || !info.Mode().IsRegular() || info.Size() > MaxBindingBytes {
		return BindingState{}, ErrStorage
	}
	f, e := os.Open(path)
	if e != nil {
		return BindingState{}, ErrStorage
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(info, actual) {
		return BindingState{}, ErrStorage
	}
	raw, e := io.ReadAll(io.LimitReader(f, MaxBindingBytes+1))
	if e != nil || len(raw) > MaxBindingBytes {
		return BindingState{}, ErrStorage
	}
	if _, e = Decode(raw); e != nil {
		return BindingState{}, ErrStorage
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var st BindingState
	if d.Decode(&st) != nil || ValidateBindingState(st) != nil {
		return BindingState{}, ErrStorage
	}
	return st, nil
}
func WriteBindingState(path string, st BindingState) error {
	if ValidateBindingState(st) != nil {
		return ErrBinding
	}
	sort.Slice(st.Bindings, func(i, j int) bool { return st.Bindings[i].Target < st.Bindings[j].Target })
	raw, e := json.Marshal(st)
	if e != nil || len(raw) > MaxBindingBytes {
		return ErrStorage
	}
	dir := filepath.Dir(path)
	di, e := os.Lstat(dir)
	if e != nil || !di.IsDir() || di.Mode()&os.ModeSymlink != 0 {
		return ErrStorage
	}
	if fi, e := os.Lstat(path); e == nil && !fi.Mode().IsRegular() || e != nil && !os.IsNotExist(e) {
		return ErrStorage
	}
	f, e := os.CreateTemp(dir, ".rf-bindings-")
	if e != nil {
		return ErrStorage
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(raw)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return ErrStorage
	}
	if e = os.Rename(tmp, path); e != nil {
		return ErrStorage
	}
	// The journal must be durable BEFORE a native write is attempted.
	df, e := os.Open(dir)
	if e != nil {
		return ErrStorage
	}
	defer df.Close()
	if e = df.Sync(); e != nil {
		return ErrStorage
	}
	return nil
}
