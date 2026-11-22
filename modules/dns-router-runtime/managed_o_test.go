package dnsrouter

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const oHash = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const oHash2 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func oLedger() LedgerO {
	return LedgerO{SchemaVersion: 1, State: "pending", Revision: oHash, Listen: "192.168.10.1:55358", Zones: []string{"alpha.dpr2n.test", "beta.dpr2n.test"}, BaselineHash: oHash2}
}
func oSnapshot(entries string) string {
	return `{"server":[{"address":"1.1.1.1","port":53,"domain":""}` + entries + `]}`
}
func TestDPR2O1NoLedgerNeverClaimsOwnership(t *testing.T) {
	s := NewLedgerStoreO(filepath.Join(t.TempDir(), "ledger.json"))
	r, e := InspectManagedOwnershipO(context.Background(), s, nil, "")
	if e != nil || r.State != "disabled" || r.NDMCInvoked || r.ChangesApplied {
		t.Fatalf("%+v %v", r, e)
	}
}
func TestDPR2O1LedgerAtomicCAS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "managed.json")
	s := NewLedgerStoreO(path)
	v := oLedger()
	sha, e := s.SaveCAS(v, "")
	if e != nil || !hashSyntaxO(sha) {
		t.Fatalf("save %s %v", sha, e)
	}
	info, e := os.Stat(path)
	if e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions %v %v", info, e)
	}
	if _, e = s.SaveCAS(v, ""); !errors.Is(e, ErrStale) {
		t.Fatalf("CAS stale %v", e)
	}
	stored, rev, e := s.Load()
	if e != nil || stored.State != "pending" || rev != sha {
		t.Fatalf("load %+v %v %v", stored, rev, e)
	}
	v.State = "active"
	_, e = s.SaveCAS(v, sha)
	if e != nil {
		t.Fatal(e)
	}
}
func TestDPR2O1PendingAndDrift(t *testing.T) {
	v := oLedger()
	both := oSnapshot(`,{"address":"192.168.10.1","port":55358,"domain":"alpha.dpr2n.test"},{"address":"192.168.10.1","port":55358,"domain":"beta.dpr2n.test"}`)
	r, e := ReconcileLedgerO(&v, oHash, both)
	if e != nil || r.State != "pending_recovery" || !r.RecoveryRequired || len(r.OwnedZones) != 2 {
		t.Fatalf("pending: %+v %v", r, e)
	}
	v.State = "active"
	r, e = ReconcileLedgerO(&v, oHash, both)
	if e != nil || r.State != "verified_active" || r.RecoveryRequired {
		t.Fatalf("active: %+v %v", r, e)
	}
	missing := oSnapshot(`,{"address":"192.168.10.1","port":55358,"domain":"beta.dpr2n.test"}`)
	r, e = ReconcileLedgerO(&v, oHash, missing)
	if e != nil || r.State != "drift" || len(r.MissingZones) != 1 || r.MissingZones[0] != "alpha.dpr2n.test" {
		t.Fatalf("drift: %+v %v", r, e)
	}
}
func TestDPR2O1ForeignOwnershipNeverAdopted(t *testing.T) {
	v := oLedger()
	v.State = "active"
	foreign := oSnapshot(`,{"address":"192.168.10.2","port":55358,"domain":"alpha.dpr2n.test"},{"address":"192.168.10.1","port":55358,"domain":"beta.dpr2n.test"}`)
	r, e := ReconcileLedgerO(&v, oHash, foreign)
	if e != nil || r.State != "drift" || len(r.ConflictZones) != 1 || len(r.OwnedZones) != 1 {
		t.Fatalf("foreign %+v %v", r, e)
	}
}
func TestDPR2O1RejectUnsafeLedgerAndCorruption(t *testing.T) {
	v := oLedger()
	v.Zones = []string{"local", "beta.dpr2n.test"}
	if validateLedgerO(v) == nil {
		t.Fatal("protected zone")
	}
	v = oLedger()
	v.Zones = []string{"alpha.dpr2n.test", "x.alpha.dpr2n.test"}
	if validateLedgerO(v) == nil {
		t.Fatal("nested zone")
	}
	v = oLedger()
	v.Listen = "192.168.10.9:55358"
	if validateLedgerO(v) == nil {
		t.Fatal("foreign host")
	}
	p := filepath.Join(t.TempDir(), "ledger.json")
	os.WriteFile(p, []byte(`{"schema_version":1, "unknown":true}`), 0600)
	s := NewLedgerStoreO(p)
	if _, _, e := s.Load(); e == nil || !strings.Contains(e.Error(), "unknown field") {
		t.Fatalf("corruption %v", e)
	}
}

type testSnapshotDriverO struct{ value string }

func (f testSnapshotDriverO) Command(_ context.Context, c string) (string, error) {
	if c != "show ip name-server" {
		return "", errors.New("unexpected")
	}
	return f.value, nil
}
func TestDPR2O1RejectStalePolicy(t *testing.T) {
	s := NewLedgerStoreO(filepath.Join(t.TempDir(), "state.json"))
	v := oLedger()
	v.State = "active"
	if _, e := s.SaveCAS(v, ""); e != nil {
		t.Fatal(e)
	}
	current := oSnapshot(`,{"address":"192.168.10.1","port":55358,"domain":"alpha.dpr2n.test"},{"address":"192.168.10.1","port":55358,"domain":"beta.dpr2n.test"}`)
	r, e := InspectManagedOwnershipO(context.Background(), s, testSnapshotDriverO{current}, oHash2)
	if e != nil || r.State != "stale_policy" || !r.RecoveryRequired || !r.NDMCInvoked {
		t.Fatalf("state %+v %v", r, e)
	}
}

func TestDPR2O1APIDisabledAndNoMutation(t *testing.T) {
	store := NewConfigStore(filepath.Join(t.TempDir(), "config.json"))
	api := NewAPI(store, "")
	ledgerPath := filepath.Join(t.TempDir(), "managed.json")
	api.LedgerO = NewLedgerStoreO(ledgerPath)
	get := httptest.NewRecorder()
	api.Handler().ServeHTTP(get, httptest.NewRequest("GET", "/v1/keenetic/managed-ownership/status", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"state":"disabled"`) {
		t.Fatalf("GET read-only status: %d %s", get.Code, get.Body.String())
	}
	post := httptest.NewRecorder()
	api.Handler().ServeHTTP(post, httptest.NewRequest("POST", "/v1/keenetic/managed-ownership/status", nil))
	if post.Code != 405 {
		t.Fatalf("POST unexpectedly accepted: %d", post.Code)
	}
	if _, e := os.Stat(ledgerPath); e == nil {
		t.Fatal("unexpected ownership file")
	}
}
