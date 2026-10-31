package lite

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/AdguardTeam/dnsproxy/upstream"
	dr "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

func TestManagerResumeAppliedSnapshotAndExplicitStop(t *testing.T) {
	var n atomic.Int32
	dest := dnsFixture(t, "OK", &n)
	c := fixtureConfig(t)
	path := filepath.Join(t.TempDir(), "config.json")
	store := dr.NewConfigStore(path)
	_, rev, e := store.Load()
	if e != nil {
		t.Fatal(e)
	}
	rev, e = store.Save(c, rev)
	if e != nil {
		t.Fatal(e)
	}
	f := func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	m := &Manager{Store: store, Path: path, Factory: f}
	if e = m.control("start", rev); e != nil {
		t.Fatal(e)
	}
	if result, err := m.Verify(rev); err != nil || result["state"] != "PASS" {
		t.Fatalf("direct service verification: %v %v", result, err)
	}
	ask(t, c.Service.Listen, "udp", "outside.example")
	c.Service.DefaultUpstream = "chosen"
	if _, e = store.Save(c, rev); e != nil {
		t.Fatal(e)
	}
	if e = m.Close(); e != nil {
		t.Fatal(e)
	}
	m2 := &Manager{Store: store, Path: path, Factory: f}
	defer m2.Close()
	if e = m2.Resume(); e != nil {
		t.Fatal(e)
	}
	if m2.revision != rev || m2.applied.Service.DefaultUpstream != "default" {
		t.Fatal("unsaved/unapplied draft was activated after restart")
	}
	_, draftRev, _ := store.Load()
	if e = m2.control("stop", draftRev); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(m2.snapshotPath()); !os.IsNotExist(e) {
		t.Fatal("stop did not disable DNS autostart")
	}
}
func TestManagerStaleRevisionAndFailedApplyKeepWorkingDNS(t *testing.T) {
	var n atomic.Int32
	dest := dnsFixture(t, "OLD", &n)
	c := fixtureConfig(t)
	path := filepath.Join(t.TempDir(), "config.json")
	store := dr.NewConfigStore(path)
	_, rev, _ := store.Load()
	rev, _ = store.Save(c, rev)
	f := func(dr.Upstream) (upstream.Upstream, error) {
		return upstream.AddressToUpstream(dest, &upstream.Options{Timeout: time.Second})
	}
	m := &Manager{Store: store, Path: path, Factory: f}
	defer m.Close()
	if e := m.control("start", "stale"); e == nil {
		t.Fatal("stale start accepted")
	}
	if e := m.control("start", rev); e != nil {
		t.Fatal(e)
	}
	// A syntactically valid, unreachable local bind must restore the previous listener.
	c.Service.Listen = "192.168.254.254:55358"
	next, e := store.Save(c, rev)
	if e != nil {
		t.Fatal(e)
	}
	if e = m.control("apply", next); e == nil {
		t.Fatal("unavailable listener accepted")
	}
	if m.engine == nil || m.revision != rev {
		t.Fatal("old service was not restored")
	}
	ask(t, m.applied.Service.Listen, "tcp", "still.example")
	if e = m.engine.Close(context.Background()); e != nil {
		t.Fatal(e)
	}
	m.engine = nil
}
