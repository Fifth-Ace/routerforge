package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = "dev"

type moduleConfig struct {
	ID          string
	DisplayName string
	Socket      string
	UIPath      string
}

var moduleConfigs = map[string]moduleConfig{
	"maintenance": {
		ID: "maintenance", DisplayName: "Maintenance",
		Socket: "/opt/var/run/routerforge-maintenance.sock",
		UIPath: "/opt/share/routerforge/modules/maintenance/ui",
	},
	"network-tools": {
		ID: "network-tools", DisplayName: "Network Tools",
		Socket: "/opt/var/run/routerforge-network-tools.sock",
		UIPath: "/opt/share/routerforge/modules/network-tools/ui",
	},
	"integrations": {
		ID: "integrations", DisplayName: "Integrations",
		Socket: "/opt/var/run/routerforge-integrations.sock",
		UIPath: "/opt/share/routerforge/modules/integrations/ui",
	},
	"developer-tools": {
		ID: "developer-tools", DisplayName: "Developer Tools",
		Socket: "/opt/var/run/routerforge-developer-tools.sock",
		UIPath: "/opt/share/routerforge/modules/developer-tools/ui",
	},
}

func main() {
	moduleID := flag.String("module", "", "RouterForge module id")
	socket := flag.String("socket", "", "Unix socket path override")
	uiPath := flag.String("ui", "", "UI directory override")
	flag.Parse()

	id := strings.TrimSpace(strings.ToLower(*moduleID))
	cfg, ok := moduleConfigs[id]
	if !ok {
		fmt.Fprintf(os.Stderr, "unsupported RouterForge vNext module %q\n", id)
		os.Exit(2)
	}
	if strings.TrimSpace(*socket) != "" {
		cfg.Socket = strings.TrimSpace(*socket)
	}
	if strings.TrimSpace(*uiPath) != "" {
		cfg.UIPath = strings.TrimSpace(*uiPath)
	}
	if err := serveModule(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", cfg.ID, err)
		os.Exit(1)
	}
}
