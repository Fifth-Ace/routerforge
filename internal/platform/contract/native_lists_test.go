package contract

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/platform/policy"
)

func TestNativeListProjection(t *testing.T) {
	raw := []byte(`{"revision":"sha256:old","config":{"schema_version":1,"lists":[{"id":"list-a","match":"suffix","upstream":"up","priority":10}],"rules":[{"id":"r","list_id":"list-a","domain":"EXAMPLE.org","match":"suffix","upstream":"up","priority":10},{"id":"other","domain":"else.example","match":"exact"}]}}`)
	l, e := DNSNativeList(raw, "list-a")
	if e != nil || len(l.Values) != 1 || l.Values[0] != "example.org" {
		t.Fatal(l, e)
	}
	other, e := DNSNativeList([]byte(strings.Replace(string(raw), "sha256:old", "sha256:new", 1)), "list-a")
	if e != nil || other.SHA256 != l.SHA256 {
		t.Fatal("unrelated revision", other, e)
	}
	changed, e := DNSNativeList([]byte(strings.Replace(string(raw), "EXAMPLE.org", "new.example", 1)), "list-a")
	if e != nil || changed.SHA256 == l.SHA256 {
		t.Fatal("source changed", changed, e)
	}
	if _, e := DNSNativeList(raw, "missing"); e == nil {
		t.Fatal("missing list")
	}
	if _, e := DNSNativeList([]byte(strings.Replace(string(raw), `"domain":"EXAMPLE.org"`, `"domain":"*.example.org"`, 1)), "list-a"); e == nil {
		t.Fatal("unsupported domain")
	}
	pin := "dns.list-a@" + l.SHA256
	r := consumerFixture(t, fmt.Sprintf(`[{"id":"native","name":"Native","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"list-ref","value":%q}]},{"id":"parent","name":"Parent","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"object","value":"native"},{"kind":"domain","value":"example.org"}]}]`, pin))
	calls := 0
	resolver := func(ref policy.NativeListReference) (NativeList, error) {
		calls++
		if ref.Module != "dns" || ref.Name != "list-a" {
			t.Fatal(ref)
		}
		return l, nil
	}
	p, e := ProjectConsumerWithNative(r, "dns", "parent", resolver)
	if e != nil || len(p.Values) != 1 || p.Values[0] != "example.org" || len(p.Blocked) != 0 || calls != 1 {
		t.Fatal(p, e, calls)
	}
	p, e = ProjectConsumerWithNative(r, "network", "parent", func(policy.NativeListReference) (NativeList, error) { return changed, nil })
	if e != nil || len(p.Values) != 0 || len(p.Blocked) != 1 || p.Blocked[0] != "NATIVE_REFERENCE_STALE" {
		t.Fatal(p, e)
	}
	p, e = ProjectConsumerWithNative(r, "dns", "parent", func(policy.NativeListReference) (NativeList, error) { return NativeList{}, ErrNativeList })
	if e != nil || len(p.Values) != 0 || len(p.Blocked) != 1 {
		t.Fatal(p, e)
	}
	refs, e := NativeReferenceIndex(r)
	if e != nil || len(refs) != 1 || refs[0].Ref != pin || strings.Join(refs[0].Groups, ",") != "native,parent" {
		t.Fatal(refs, e)
	}
	unpinned := consumerFixture(t, `[{"id":"unpin","name":"Unpin","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"list-ref","value":"dns.list-a"}]}]`)
	p, e = ProjectConsumerWithNative(unpinned, "dns", "unpin", resolver)
	if e != nil || p.Blocked[0] != "NATIVE_REFERENCE_UNPINNED" || calls != 1 {
		t.Fatal(p, e, calls)
	}
}
func TestNativeNFQWSList(t *testing.T) {
	body := func(name, content, sha string) []byte {
		b, e := json.Marshal(map[string]any{"name": name, "content": content, "size": len(content), "sha256": sha, "path": "never-return-this"})
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	content := "# comment\r\nExample.org\nexample.org\nother.example\n"
	hash := nativeDigest([]byte(content))
	l, e := NFQWSNativeList(body("user.list", content, hash), "user.list")
	if e != nil || l.SHA256 != hash || strings.Join(l.Values, ",") != "example.org,other.example" {
		t.Fatal(l, e)
	}
	for _, s := range []string{"1.1.1.1", "192.0.2.0/24", "*.example.org", "||example.org^", "example.org # comment", "bad/name"} {
		if _, e := NFQWSNativeList(body("user.list", s, nativeDigest([]byte(s))), "user.list"); e == nil {
			t.Fatal("accepted", s)
		}
	}
	if _, e := NFQWSNativeList(body("user.list", content, strings.Repeat("0", 64)), "user.list"); e == nil {
		t.Fatal("bad digest")
	}
	if _, e := NFQWSNativeList(body("user.list-old", content, hash), "user.list-old"); e == nil {
		t.Fatal("backup")
	}
	for _, s := range []string{"dns.../x", "nfqws...list", "nfqws.user.list-old", "dns.x@aaa", "dns.x@" + strings.Repeat("F", 64), "https://example.org"} {
		if _, ok := policy.ParseNativeListReference(s); ok {
			t.Fatal("accepted invalid ref", s)
		}
	}
}
func TestNativeReferenceBudget(t *testing.T) {
	sha := strings.Repeat("a", 64)
	members := []policy.Member{}
	for i := 0; i < NativeReferenceBudget+1; i++ {
		members = append(members, policy.Member{Kind: "list-ref", Value: fmt.Sprintf("dns.list-%d@%s", i, sha)})
	}
	b, _ := json.Marshal([]policy.Group{{ID: "many", Type: policy.DomainGroup, Name: "Many", Scope: "local", Revision: 1, Members: members}})
	r := consumerFixture(t, string(b))
	calls := 0
	p, e := ProjectConsumerWithNative(r, "dns", "many", func(policy.NativeListReference) (NativeList, error) {
		calls++
		return NativeList{sha, []string{"example.org"}}, nil
	})
	if e != nil || calls != NativeReferenceBudget || len(p.Values) != 0 || len(p.Blocked) != 1 || p.Blocked[0] != "NATIVE_REFERENCE_LIMIT" {
		t.Fatal(p, e, calls)
	}
}
