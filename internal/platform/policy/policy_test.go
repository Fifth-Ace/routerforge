package policy

import "testing"

func TestObjectValidate(t *testing.T) {
	good := Object{ID: "lan-main", Type: Network, Name: "Main LAN"}
	if err := good.Validate(); err != nil {
		t.Fatalf("valid object rejected: %v", err)
	}
	bad := Object{ID: "../../etc", Type: Network, Name: "bad"}
	if err := bad.Validate(); err == nil {
		t.Fatal("unsafe id accepted")
	}
}
