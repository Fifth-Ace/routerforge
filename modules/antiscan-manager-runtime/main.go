package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var version = "dev"

type runtimeConfig struct {
	Socket         string
	UIPath         string
	AntiscanDir    string
	InitScript     string
	StatusFile     string
	ConfigLockFile string
	GeoLockFile    string
}

func main() {
	socket := flag.String("socket", "/opt/var/run/routerforge-antiscan-manager.sock", "Unix socket path")
	uiPath := flag.String("ui", "/opt/share/routerforge/modules/antiscan-manager/ui", "UI directory")
	antiscanDir := flag.String("antiscan-dir", "/opt/etc/antiscan", "Antiscan configuration directory")
	initScript := flag.String("antiscan-init", "/opt/etc/init.d/S99ascn", "Antiscan init script")
	statusFile := flag.String("antiscan-status", "/tmp/ascn.run", "Antiscan runtime marker")
	configLock := flag.String("antiscan-config-lock", "/tmp/ascn.lock", "Antiscan config reload marker")
	geoLock := flag.String("antiscan-geo-lock", "/tmp/ascn_geo.lock", "Antiscan geo reload marker")
	flag.Parse()

	cfg := runtimeConfig{
		Socket:         strings.TrimSpace(*socket),
		UIPath:         strings.TrimSpace(*uiPath),
		AntiscanDir:    strings.TrimSpace(*antiscanDir),
		InitScript:     strings.TrimSpace(*initScript),
		StatusFile:     strings.TrimSpace(*statusFile),
		ConfigLockFile: strings.TrimSpace(*configLock),
		GeoLockFile:    strings.TrimSpace(*geoLock),
	}
	if cfg.Socket == "" || cfg.UIPath == "" || cfg.AntiscanDir == "" || cfg.InitScript == "" || cfg.StatusFile == "" {
		fmt.Fprintln(os.Stderr, "antiscan-manager: socket, ui and Antiscan paths are required")
		os.Exit(2)
	}
	if err := serveAntiscanManager(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "antiscan-manager: %v\n", err)
		os.Exit(1)
	}
}
