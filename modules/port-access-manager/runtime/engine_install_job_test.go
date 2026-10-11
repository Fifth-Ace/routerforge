package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEngineInstallJobAsyncAndSerialized(t *testing.T) {
	manager := &engineInstallManager{}
	started := make(chan struct{})
	finish := make(chan struct{})
	if err := manager.start("knockd", func(ctx context.Context, pkg string) error {
		close(started)
		<-finish
		if pkg != "knockd" {
			t.Errorf("unexpected package: %s", pkg)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	<-started
	if got := manager.status("knockd").State; got != "running" {
		t.Fatalf("status %s", got)
	}
	if err := manager.start("fwknopd", func(context.Context, string) error { return nil }); err == nil {
		t.Fatal("concurrent opkg accepted")
	}
	close(finish)
	deadline := time.After(time.Second)
	for manager.status("knockd").State != "completed" {
		select {
		case <-deadline:
			t.Fatal("job did not finish")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if err := manager.start("../bad", func(context.Context, string) error { return nil }); err == nil {
		t.Fatal("unknown engine accepted")
	}
	if err := manager.start("fwknopd", func(context.Context, string) error { return errors.New("test error") }); err != nil {
		t.Fatal(err)
	}
	deadline = time.After(time.Second)
	for manager.status("fwknopd").State == "running" {
		select {
		case <-deadline:
			t.Fatal("failure did not finish")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if manager.status("fwknopd").State != "failed" {
		t.Fatal("failure not reported")
	}
}
