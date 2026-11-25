package contract

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBindingState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory durability tested in Linux CI")
	}
	p := filepath.Join(t.TempDir(), "bindings.json")
	st, e := ReadBindingState(p)
	if e != nil || st.Generation != 0 || len(st.Bindings) != 0 {
		t.Fatal(st, e)
	}
	if _, e = os.Stat(p); !os.IsNotExist(e) {
		t.Fatal("read wrote registry")
	}
	v := []string{"old.example"}
	n := []string{"new.example"}
	b := Binding{Target: "nfqws.rf-binding-demo.list", GroupID: "root", Checkpoint: BindingSnapshot{BindingTextDigest(v), v}, Pending: &BindingPending{"sync", BindingSnapshot{BindingTextDigest(v), v}, BindingSnapshot{BindingTextDigest(n), n}}}
	st.Bindings = append(st.Bindings, b)
	st.Generation++
	if e = WriteBindingState(p, st); e != nil {
		t.Fatal(e)
	}
	got, e := ReadBindingState(p)
	if e != nil || got.Bindings[0].Pending == nil || got.Bindings[0].Pending.After.SHA256 != b.Pending.After.SHA256 {
		t.Fatal(got, e)
	}
	if e = os.WriteFile(p, []byte(`{"schemaVersion":1,"schemaVersion":1,"generation":0,"bindings":[]}`), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = ReadBindingState(p); e == nil {
		t.Fatal("duplicate key accepted")
	}
}
func TestBindingDependency(t *testing.T) {
	for _, s := range []string{"dns.user", "nfqws.user.list", "nfqws.rf-binding-.list", "nfqws.rf-binding-a.list-old", "dns.rf-binding-a@" + strings.Repeat("a", 64), "dns.rf-binding-a..b"} {
		if _, ok := BindingTarget(s); ok {
			t.Fatal(s)
		}
	}
	v := []string{"old.example"}
	st := BindingState{1, 0, []Binding{{Target: "nfqws.rf-binding-demo.list", GroupID: "root", Checkpoint: BindingSnapshot{BindingTextDigest(v), v}}}}
	cat := func(groups string) Revision {
		return Revision{Catalog: []byte(`{"schemaVersion":1,"devices":[],"connections":[],"checkPolicies":[],"policyObjects":[` + groups + `]}`)}
	}
	g := `{"id":"root","type":"domain-group","name":"Group","scope":"local","revision":1,"members":[{"kind":"domain","value":"old.example"}]}`
	if e := CheckBindingCatalog(cat(g), st); e != nil {
		t.Fatal(e)
	}
	if CheckBindingCatalog(cat(""), st) == nil {
		t.Fatal("deleted binding dependency")
	}
	g = strings.Replace(g, `"kind":"domain","value":"old.example"`, `"kind":"list-ref","value":"nfqws.rf-binding-demo.list@`+strings.Repeat("a", 64)+`"`, 1)
	if CheckBindingCatalog(cat(g), st) == nil {
		t.Fatal("self reference accepted")
	}
}
