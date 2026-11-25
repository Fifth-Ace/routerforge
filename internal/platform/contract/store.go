package contract

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

const MaxCatalogBytes = 256 << 10
const HistoryLimit = 4

var ErrConflict = errors.New("catalog revision conflict")
var ErrInvalid = errors.New("invalid draft catalog")
var ErrStorage = errors.New("catalog storage unavailable")
var emptyCatalog = []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[]}`)

type Revision struct {
	Generation int64           `json:"generation"`
	ETag       string          `json:"etag"`
	Catalog    json.RawMessage `json:"catalog"`
}
type State struct {
	Current Revision   `json:"current"`
	History []Revision `json:"history"`
}
type Preview struct {
	Candidate Revision                   `json:"candidate"`
	Added     []string                   `json:"added"`
	Changed   []string                   `json:"changed"`
	Removed   []string                   `json:"removed"`
	Expanded  map[string][]policy.Member `json:"expanded"`
}
type Store struct {
	mu   sync.Mutex
	path string
}

func NewStore(path string) *Store { return &Store{path: path} }
func revision(b []byte, g int64) Revision {
	h := sha256.Sum256(append([]byte(strconv.FormatInt(g, 10)+":"), b...))
	return Revision{g, hex.EncodeToString(h[:]), append(json.RawMessage(nil), b...)}
}
func canonical(b []byte) ([]byte, error) {
	if len(b) > MaxCatalogBytes {
		return nil, ErrInvalid
	}
	if e := ValidateCatalog(b); e != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, e)
	}
	v, e := Decode(b)
	if e != nil {
		return nil, ErrInvalid
	}
	c := obj(v)
	// The first Core catalogue is metadata-only. Credentials/trust and non-root
	// paths are not yet stored or accepted, even as references. P3/P4 adds vault.
	for _, v := range arr(c["connections"]) {
		x := obj(v)
		if x["credentialRef"] != nil {
			return nil, ErrInvalid
		}
		for _, v := range arr(x["endpoints"]) {
			ep := obj(v)
			if ep["trustProfileRef"] != nil {
				return nil, ErrInvalid
			}
			if p, ok := ep["publicPath"].(string); ok && p != "/" {
				return nil, ErrInvalid
			}
		}
	}
	out, e := json.Marshal(v)
	if e != nil || len(out) > MaxCatalogBytes {
		return nil, ErrInvalid
	}
	return out, nil
}
func (s *Store) load() (State, error) {
	info, e := os.Lstat(s.path)
	if os.IsNotExist(e) {
		b, _ := canonical(emptyCatalog)
		return State{revision(b, 0), []Revision{}}, nil
	}
	if e != nil || !info.Mode().IsRegular() || info.Size() > int64((HistoryLimit+1)*MaxCatalogBytes+8192) {
		return State{}, ErrStorage
	}
	f, e := os.Open(s.path)
	if e != nil {
		return State{}, ErrStorage
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64((HistoryLimit+1)*MaxCatalogBytes+8193)))
	if e != nil {
		return State{}, ErrStorage
	}
	var st State
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&st) != nil || len(st.History) > HistoryLimit {
		return State{}, ErrStorage
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return State{}, ErrStorage
	}
	for _, r := range append([]Revision{st.Current}, st.History...) {
		b, e := canonical(r.Catalog)
		if e != nil || r.Generation < 0 || r.Generation > 9007199254740991 || revision(b, r.Generation).ETag != r.ETag {
			return State{}, ErrStorage
		}
	}
	for i, h := range st.History {
		if h.Generation != st.Current.Generation-int64(i)-1 {
			return State{}, ErrStorage
		}
	}
	if st.History == nil {
		st.History = []Revision{}
	}
	return st, nil
}
func (s *Store) Get() (State, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.load() }
func entities(b []byte) map[string]map[string]any {
	v, _ := Decode(b)
	c := obj(v)
	out := map[string]map[string]any{}
	for _, k := range []string{"devices", "connections", "checkPolicies", "policyObjects"} {
		for _, v := range arr(c[k]) {
			x := obj(v)
			id := "id"
			if k == "checkPolicies" {
				id = "connectionId"
			}
			out[k+":"+str(x[id])] = x
		}
	}
	return out
}
func comparable(x map[string]any) []byte { b, _ := json.Marshal(x); return b }
func compare(st State, input []byte) (Preview, error) {
	b, e := canonical(input)
	if e != nil {
		return Preview{}, e
	}
	old := entities(st.Current.Catalog)
	v, _ := Decode(b)
	c := obj(v)
	// Assign revisions server-side, preserving IDs. A stale whole-catalog ETag is
	// rejected before this function. Imported revision counters are never trusted.
	for _, k := range []string{"devices", "policyObjects"} {
		for _, v := range arr(c[k]) {
			x := obj(v)
			key := k + ":" + str(x["id"])
			prev := old[key]
			if prev == nil {
				x["revision"] = float64(1)
				continue
			}
			x["revision"] = prev["revision"]
			if !bytes.Equal(comparable(x), comparable(prev)) {
				n := prev["revision"].(float64)
				if n >= 9007199254740991 {
					return Preview{}, ErrInvalid
				}
				x["revision"] = n + 1
			}
		}
	}
	b, _ = json.Marshal(c)
	if len(b) > MaxCatalogBytes {
		return Preview{}, ErrInvalid
	}
	now := entities(b)
	gen := st.Current.Generation
	if !bytes.Equal(b, st.Current.Catalog) {
		if gen >= 9007199254740991 {
			return Preview{}, ErrInvalid
		}
		gen++
	}
	p := Preview{Candidate: revision(b, gen), Added: []string{}, Changed: []string{}, Removed: []string{}, Expanded: map[string][]policy.Member{}}
	// Stable diff order, only entity identifiers (never values) in impact lists.
	for k, x := range now {
		y := old[k]
		if y == nil {
			p.Added = append(p.Added, k)
		} else if !bytes.Equal(comparable(x), comparable(y)) {
			p.Changed = append(p.Changed, k)
		}
	}
	for k := range old {
		if now[k] == nil {
			p.Removed = append(p.Removed, k)
		}
	}
	sort.Strings(p.Added)
	sort.Strings(p.Changed)
	sort.Strings(p.Removed)
	gBytes, _ := json.Marshal(c["policyObjects"])
	var g []policy.Group
	_ = json.Unmarshal(gBytes, &g)
	total := 0
	for _, x := range g {
		a, e := policy.Expand(g, x.ID)
		if e != nil {
			return Preview{}, ErrInvalid
		}
		total += len(a)
		if total > 8192 {
			return Preview{}, ErrInvalid
		}
		p.Expanded[x.ID] = a
	}
	return p, nil
}
func (s *Store) Preview(etag string, b []byte) (Preview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, e := s.load()
	if e != nil {
		return Preview{}, e
	}
	if st.Current.ETag != etag {
		return Preview{}, ErrConflict
	}
	return compare(st, b)
}

// Commit saves only the declarative catalogue, after a separately confirmed preview.
func (s *Store) Commit(etag, previewETag string, b []byte) (State, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, e := s.load()
	if e != nil {
		return State{}, false, e
	}
	if st.Current.ETag != etag {
		return State{}, false, ErrConflict
	}
	p, e := compare(st, b)
	if e != nil {
		return State{}, false, e
	}
	if p.Candidate.ETag != previewETag {
		return State{}, false, ErrConflict
	}
	if p.Candidate.ETag == etag {
		return st, false, nil
	}
	st.History = append([]Revision{st.Current}, st.History...)
	if len(st.History) > HistoryLimit {
		st.History = st.History[:HistoryLimit]
	}
	st.Current = p.Candidate
	data, e := json.Marshal(st)
	if e != nil {
		return State{}, false, ErrStorage
	}
	dir := filepath.Dir(s.path)
	di, e := os.Lstat(dir)
	if e != nil || !di.IsDir() || di.Mode()&os.ModeSymlink != 0 {
		return State{}, false, ErrStorage
	}
	f, e := os.CreateTemp(dir, ".rf-policy-")
	if e != nil {
		return State{}, false, ErrStorage
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(data); e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil || ce != nil {
		return State{}, false, ErrStorage
	}
	if e = os.Rename(tmp, s.path); e != nil {
		return State{}, false, ErrStorage
	}
	return st, true, nil
}
