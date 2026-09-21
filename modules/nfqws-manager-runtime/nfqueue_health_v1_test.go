package main

import (
	"reflect"
	"testing"
)

func TestBuildV2NFQueueHealthHealthy(t *testing.T) {
	got := buildV2NFQueueHealth(true, true, true, []int{300, 30000}, []int{300, 30000}, []int{300, 30000}, "sha")
	if !got.OK || got.State != "HEALTHY" {
		t.Fatalf("health=%+v", got)
	}
	if !reflect.DeepEqual(got.ProductionQueues, []int{300}) {
		t.Fatalf("production queues=%v", got.ProductionQueues)
	}
	if len(got.MissingKernelQueues) != 0 || len(got.MissingFirewallQueues) != 0 {
		t.Fatalf("unexpected missing queues=%+v", got)
	}
}

func TestBuildV2NFQueueHealthDetectsMissingKernelBinding(t *testing.T) {
	got := buildV2NFQueueHealth(true, true, true, []int{300}, []int{300}, []int{}, "sha")
	if got.OK || got.State != "UNHEALTHY" {
		t.Fatalf("health=%+v", got)
	}
	if !reflect.DeepEqual(got.MissingKernelQueues, []int{300}) {
		t.Fatalf("missing kernel=%v", got.MissingKernelQueues)
	}
}

func TestBuildV2NFQueueHealthInconclusiveInventory(t *testing.T) {
	got := buildV2NFQueueHealth(true, false, true, []int{300}, nil, []int{300}, "sha")
	if got.OK || got.State != "INCONCLUSIVE" {
		t.Fatalf("health=%+v", got)
	}
}
