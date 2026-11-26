package storageobserve

import (
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	m := Parse(strings.NewReader("8 0 sda 10 0 20 30 40 0 50 60 0 70 80\n8 1 bad X 0 1 2 3 4 5 6 7 8 9\n"))
	if m["sda"] != (Counters{10, 20, 30, 40, 50, 60, 70}) || len(m) != 1 {
		t.Fatal(m)
	}
}
func TestDelta(t *testing.T) {
	a := Counters{1, 2, 3, 4, 5, 6, 7}
	b := Counters{3, 6, 13, 4, 5, 6, 507}
	v := Delta(a, b, time.Second)
	if v == nil || v.ReadBPS != 2048 || v.BusyPct != 50 || *v.ReadAwaitMS != 5 || v.WriteAwaitMS != nil {
		t.Fatal(v)
	}
	if Delta(b, a, time.Second) != nil || Delta(a, b, 0) != nil || Delta(a, b, 6*time.Minute) != nil {
		t.Fatal("invalid delta")
	}
}
func TestIdentityAndCapacity(t *testing.T) {
	var tr Tracker
	now := time.Unix(1000, 0)
	id := map[string]string{"sda": "hw:a"}
	m := map[string][]Mount{"sda": {{"/opt", 1000, 800}}}
	for i := 0; i < 4; i++ {
		m["sda"][0].Available = 800 - uint64(i*100)
		v := tr.Observe(now.Add(time.Duration(i)*15*time.Minute), id, map[string]Counters{"sda": {Reads: uint64(i)}}, m)
		if i == 3 && v["sda"].Capacity["/opt"].EstimatedDays == nil {
			t.Fatal(v)
		}
	}
	id["sda"] = "hw:b"
	v := tr.Observe(now.Add(60*time.Minute), id, map[string]Counters{"sda": {}}, m)
	if v["sda"].IO != nil || len(v["sda"].Capacity["/opt"].Points) != 1 {
		t.Fatal(v)
	}
	id["sda"] = "volatile:sda"
	if v = tr.Observe(now.Add(61*time.Minute), id, nil, m); v["sda"].IdentityStable || len(v["sda"].Capacity) != 0 {
		t.Fatal(v)
	}
}
func TestDuplicateIDAndDisappear(t *testing.T) {
	var tr Tracker
	now := time.Now()
	id := map[string]string{"sda": "hw:same", "sdb": "hw:same"}
	if tr.Observe(now, id, nil, nil)["sda"].IdentityStable {
		t.Fatal("duplicate")
	}
	tr.Observe(now, map[string]string{"sda": "hw:a"}, map[string]Counters{"sda": {}}, nil)
	tr.Observe(now.Add(time.Second), nil, nil, nil)
	if tr.Observe(now.Add(2*time.Second), map[string]string{"sda": "hw:a"}, map[string]Counters{"sda": {Reads: 1}}, nil)["sda"].IO != nil {
		t.Fatal("joined disconnected device")
	}
}
