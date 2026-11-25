package contract

import (
	"encoding/json"
	"testing"
)

func consumerFixture(t *testing.T, extra string) Revision {
	t.Helper()
	b := sampleCatalog(t)
	v, e := Decode(b)
	if e != nil {
		t.Fatal(e)
	}
	c := obj(v)
	var g []any
	if e = json.Unmarshal([]byte(extra), &g); e != nil {
		t.Fatal(e)
	}
	c["policyObjects"] = append(arr(c["policyObjects"]), g...)
	b, e = json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	if e = ValidateCatalog(b); e != nil {
		t.Fatal(e)
	}
	return revision(b, 1)
}
func TestConsumerProjection(t *testing.T) {
	r := consumerFixture(t, `[{"id":"domain-a","name":"Domains","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"domain","value":"example.org"}]},{"id":"domain-b","name":"Nested","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"object","value":"domain-a"},{"kind":"domain","value":"example.org"}]},{"id":"ref","name":"Ref","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"object","value":"domain-a"},{"kind":"list-ref","value":"existing"}]},{"id":"cidr","name":"Net","type":"network","scope":"local","revision":1,"members":[{"kind":"cidr","value":"192.0.2.0/24"}]},{"id":"wild","name":"Wild","type":"domain-group","scope":"local","revision":1,"members":[{"kind":"domain","value":"*.example.org"}]}]`)
	for _, c := range []struct {
		consumer, id string
		want         string
		blocked      bool
	}{{"dns", "domain-b", "example.org", false}, {"nfqws-domain", "domain-a", "example.org", false}, {"dns", "ref", "", true}, {"dns", "wild", "", true}, {"network", "cidr", "", true}, {"nfqws-ip", "cidr", "192.0.2.0/24", false}, {"network", "lab-vps", "vps.example", false}} {
		p, e := ProjectConsumer(r, c.consumer, c.id)
		if e != nil {
			t.Fatal(e)
		}
		if p.ETag != r.ETag {
			t.Fatal("revision")
		}
		if c.blocked {
			if len(p.Blocked) == 0 || len(p.Values) != 0 {
				t.Fatal(p)
			}
		} else if len(p.Values) != 1 || p.Values[0] != c.want || len(p.Blocked) != 0 {
			t.Fatal(p)
		}
	}
	if _, e := ProjectConsumer(r, "dns", "cidr"); e == nil {
		t.Fatal("incompatible")
	}
	if _, e := ConsumerGroups(r, "evil"); e == nil {
		t.Fatal("unknown consumer")
	}
	g, e := ConsumerGroups(r, "nfqws-ip")
	if e != nil || len(g) != 1 || g[0].ID != "cidr" {
		t.Fatal(g, e)
	}
}
