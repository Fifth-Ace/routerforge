package main

import "testing"

func TestApplyGateStatusZeroProfileIdentity(t *testing.T) {
	status := benchAutoTuneApplyGateStatus{
		Eligible:           true,
		ServerName:         "www.googlevideo.com",
		SourceProfileIndex: 0,
		ConfigSHA256:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Reason:             "test",
	}
	if !status.Eligible || status.SourceProfileIndex != 0 {
		t.Fatalf("status=%+v", status)
	}
}
