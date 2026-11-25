package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSharedFixtures(t *testing.T) {
	b, e := os.ReadFile("golden.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = ValidateDocument(b); e != nil {
		t.Fatal(e)
	}
	v, e := Decode(b)
	if e != nil {
		t.Fatal(e)
	}
	m := obj(obj(arr(obj(v)["snapshots"])[0])["metrics"])
	mem := obj(m["memory"])
	if mem["usedBytes"].(float64)/mem["totalBytes"].(float64)*100 != 25 || obj(m["swap"])["totalBytes"] != float64(0) || obj(arr(m["network"])[0])["rxBps"] != nil {
		t.Fatal("metric projection mismatch")
	}
	b, e = os.ReadFile("negative.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []struct {
		Name     string          `json:"name"`
		Document json.RawMessage `json:"document"`
	}
	if e = json.Unmarshal(b, &cases); e != nil {
		t.Fatal(e)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if e := ValidateDocument(c.Document); e == nil {
				t.Fatal("invalid document accepted")
			}
		})
	}
	for _, s := range []string{`{"a":1,"a":2}`, `{} {}`, strings.Repeat("[", 40) + strings.Repeat("]", 40)} {
		if _, e := Decode([]byte(s)); e == nil {
			t.Fatal("invalid JSON accepted")
		}
	}
}
func sampleCatalog(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("golden.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Catalog json.RawMessage `json:"catalog"`
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	return bytes.ReplaceAll(f.Catalog, []byte(`"/api"`), []byte(`"/"`))
}
func TestStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "draft.json")
	s := NewStore(path)
	st, e := s.Get()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("GET wrote data")
	}
	cat := sampleCatalog(t)
	p, e := s.Preview(st.Current.ETag, cat)
	if e != nil || len(p.Added) != 4 {
		t.Fatalf("preview: %+v %v", p, e)
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("preview wrote data")
	}
	if _, _, e = s.Commit(st.Current.ETag, "wrong", cat); !errors.Is(e, ErrConflict) {
		t.Fatal("wrong preview accepted")
	}
	next, changed, e := s.Commit(st.Current.ETag, p.Candidate.ETag, cat)
	if e != nil || !changed || next.Current.Generation != 1 || len(next.History) != 1 {
		t.Fatalf("commit: %+v %v", next, e)
	}
	if _, e = s.Preview(st.Current.ETag, cat); !errors.Is(e, ErrConflict) {
		t.Fatal("stale revision accepted")
	}
	restored, e := NewStore(path).Get()
	if e != nil || restored.Current.ETag != next.Current.ETag {
		t.Fatal("reload failed", e)
	}
	p, e = s.Preview(next.Current.ETag, cat)
	if e != nil || p.Candidate.ETag != next.Current.ETag {
		t.Fatal("no-op preview failed", e)
	}
	_, changed, e = s.Commit(next.Current.ETag, p.Candidate.ETag, cat)
	if e != nil || changed {
		t.Fatal("no-op write")
	}
	changedCat := bytes.ReplaceAll(cat, []byte("Lab VPS"), []byte("Changed VPS"))
	p, e = s.Preview(next.Current.ETag, changedCat)
	if e != nil || len(p.Changed) != 1 || !bytes.Contains(p.Candidate.Catalog, []byte(`"revision":2`)) {
		t.Fatal("object revision failed", e)
	}
	next, _, e = s.Commit(next.Current.ETag, p.Candidate.ETag, changedCat)
	if e != nil {
		t.Fatal(e)
	}
	p, e = s.Preview(next.Current.ETag, st.Current.Catalog)
	if e != nil {
		t.Fatal(e)
	}
	back, _, e := s.Commit(next.Current.ETag, p.Candidate.ETag, st.Current.Catalog)
	if e != nil || back.Current.ETag == st.Current.ETag || back.Current.Generation != 3 {
		t.Fatal("ABA revision regression", e)
	}
	if e = os.WriteFile(path, []byte("corrupt"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(); !errors.Is(e, ErrStorage) {
		t.Fatal("corrupt data silently accepted")
	}
}
func TestStoreSecretAndDangling(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "draft.json"))
	st, _ := s.Get()
	cat := sampleCatalog(t)
	for _, b := range [][]byte{bytes.ReplaceAll(cat, []byte(`"/"`), []byte(`"/private-secret"`)), bytes.ReplaceAll(cat, []byte(`"credentialRef": null`), []byte(`"credentialRef": "00000000-0000-4000-8000-000000000009"`)), bytes.ReplaceAll(cat, []byte(`"deviceId": "00000000-0000-4000-8000-000000000001"`), []byte(`"deviceId": "00000000-0000-4000-8000-000000000099"`))} {
		if _, e := s.Preview(st.Current.ETag, b); !errors.Is(e, ErrInvalid) {
			t.Fatal("unsafe draft accepted", e)
		}
	}
}
