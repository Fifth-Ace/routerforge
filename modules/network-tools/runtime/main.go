package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = "dev"

type runtimeConfig struct {
	Socket string
	UIPath string
}

func main() {
	socket := flag.String("socket", "/opt/var/run/routerforge-network-tools.sock", "Unix socket path")
	uiPath := flag.String("ui", "/opt/share/routerforge/modules/network-tools/ui", "UI directory")
	flag.Parse()

	cfg := runtimeConfig{
		Socket: strings.TrimSpace(*socket),
		UIPath: strings.TrimSpace(*uiPath),
	}
	if cfg.Socket == "" || cfg.UIPath == "" {
		fmt.Fprintln(os.Stderr, "network-tools: socket and ui paths are required")
		os.Exit(2)
	}
	if err := serveNetworkTools(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "network-tools: %v\n", err)
		os.Exit(1)
	}
}
