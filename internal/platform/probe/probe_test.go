package probe

import (
	"context"
	"testing"
	"time"
)

func TestResolveLiteralIP(t *testing.T) {
	got := Resolve(context.Background(), "127.0.0.1", 50*time.Millisecond)
	if !got.OK || len(got.Addresses) != 1 || got.Addresses[0] != "127.0.0.1" {
		t.Fatalf("unexpected literal result: %#v", got)
	}
}

func TestResolveRejectsEmpty(t *testing.T) {
	got := Resolve(context.Background(), "", 50*time.Millisecond)
	if got.OK || got.ErrorClass != "invalid-target" {
		t.Fatalf("unexpected result: %#v", got)
	}
}
