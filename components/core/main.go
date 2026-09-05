package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

var version = "dev"

func main() {
	listen := flag.String("listen", ":2233", "web listen address")
	// Kept for CLI/init-script compatibility with RouterForge 0.3.x. DNS scheduling
	// moved to routerforge-dns in the Module ABI v1 generation.
	_ = flag.Duration("discovery", 60*time.Second, "deprecated: DNS discovery interval is owned by routerforge-dns")
	_ = flag.Duration("health", 30*time.Second, "deprecated: DNS health interval is owned by routerforge-dns")
	logPath := flag.String("log", "/opt/var/log/routerforge.log", "core event log path")
	flag.Parse()

	if os.Geteuid() != 0 {
		log.Fatal("routerforge must run as root (package lifecycle and local module sockets require it)")
	}

	coreLogEvent(*logPath, "START", fmt.Sprintf("routerforge v%s listen=%s module-abi=v1", version, *listen))
	startOptionalProfiling()

	fmt.Printf("RouterForge v%s\n", version)
	fmt.Printf("web listen: %s\n", *listen)
	fmt.Printf("module ABI: v1\n")
	fmt.Printf("event log: %s\n", *logPath)

	if err := startWeb(*listen, version); err != nil {
		coreLogEvent(*logPath, "WEB_ERROR", err.Error())
		log.Fatal(err)
	}
}
