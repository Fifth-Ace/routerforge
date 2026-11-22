//go:build dns_router_lite

package dnsrouter

import (
	"context"
	"errors"
)

// The independent DEV executable uses DNSProxy's current QUIC stack. Its
// management API imports the shared model without linking legacy shadow QUIC.
type doqClientP8 struct{}

func newDoQClientP8() *doqClientP8 { return &doqClientP8{} }
func (*doqClientP8) close()        {}
func (*doqClientP8) exchange(context.Context, Upstream, ShadowOptions, []byte) ([]byte, error) {
	return nil, errors.New("legacy shadow DoQ is unavailable in the independent DNS executable")
}
