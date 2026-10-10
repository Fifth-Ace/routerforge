package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = "dev"

func main() {
	socket := flag.String("socket", "/opt/var/run/routerforge-port-access-manager.sock", "Unix socket")
	ui := flag.String("ui", "/opt/share/routerforge/modules/port-access-manager/ui", "UI path")
	flag.Parse()
	if strings.TrimSpace(*socket) == "" || strings.TrimSpace(*ui) == "" {
		fmt.Fprintln(os.Stderr, "socket and UI path are required")
		os.Exit(2)
	}
	if err := serve(*socket, *ui); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
