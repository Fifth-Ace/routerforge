package main

import "testing"

func TestAdminFileCopyRequestConfirmationShape(t *testing.T) {
	request := adminFileCopyRequest{
		Source:             "/opt/a",
		Destination:        "/tmp/a",
		ConfirmSource:      "/opt/a",
		ConfirmDestination: "/tmp/a",
	}
	if request.Source != request.ConfirmSource || request.Destination != request.ConfirmDestination {
		t.Fatal("copy confirmation shape mismatch")
	}
}
