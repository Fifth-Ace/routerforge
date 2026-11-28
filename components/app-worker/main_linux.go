//go:build linux

// This binary is separate from Core, so the Core service's stop action does
// not target the independent package executor. It has no network listener.
package main

import (
	"context"
	"log"
	"os"

	"github.com/Fifth-Ace/routerforge/internal/apptransaction"
)

var version = "dev"

func main() {
	if len(os.Args) != 1 || !apptransaction.DeviceExecutionConfigured(version) {
		log.Fatal("RF_APP_WORKER: execution not accepted")
	}
	c, err := apptransaction.DeviceCoordinator()
	if err != nil {
		log.Fatal(err)
	}
	j, err := c.RunAcceptedDeviceWorker(context.Background(), version)
	if err != nil {
		log.Fatalf("RF_APP_WORKER: phase=%s error=%v", j.Phase, err)
	}
	if j.Phase != apptransaction.Completed {
		log.Fatal("RF_APP_WORKER: transaction not completed")
	}
	log.Print("RF_APP_WORKER: completed")
}
