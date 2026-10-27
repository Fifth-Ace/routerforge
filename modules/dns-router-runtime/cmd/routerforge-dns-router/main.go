package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	dnsrouter "github.com/Fifth-Ace/routerforge/modules/dns-router-runtime"
)

var version = "dev"

func main() {
	config := flag.String("config", "", "path to v1 DNS Router config")
	domain := flag.String("evaluate", "", "offline evaluation of one DNS name")
	showVersion := flag.Bool("version", false, "print build version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}
	if *config == "" {
		fmt.Fprintln(os.Stderr, "--config required; RouterForge DNS Router Stage A never starts a listener")
		os.Exit(2)
	}
	engine, err := dnsrouter.Load(*config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "CONFIG_FAIL:", err)
		os.Exit(1)
	}
	if *domain == "" {
		fmt.Printf("CONFIG_PASS: %d rules; %d upstream pools; inactive, no sockets opened\n", engine.RuleCount(), len(engine.UpstreamIDs()))
		return
	}
	d, err := engine.Evaluate(*domain)
	if err != nil {
		fmt.Fprintln(os.Stderr, "EVALUATION_FAIL:", err)
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(d)
}
