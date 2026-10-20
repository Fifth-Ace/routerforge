package main

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// System ndmc must never load Entware's OpenSSL or preloaded libraries.
// This applies to the child only; other RouterForge processes are unchanged.
func systemNDMCEnv(parent []string) []string {
	out := make([]string, 0, len(parent)+1)
	for _, entry := range parent {
		key, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch key {
		case "LD_LIBRARY_PATH", "LD_PRELOAD", "LD_AUDIT", "LD_DEBUG", "LD_DEBUG_OUTPUT":
			continue
		}
		out = append(out, entry)
	}
	return append(out, "LD_LIBRARY_PATH=/lib:/usr/lib")
}

func runSystemDNSNDMC(ctx context.Context, program string, args ...string) ([]byte, error) {
	// Preserve the injectable runner signature, but forbid path substitution.
	cmd := exec.CommandContext(ctx, "/bin/ndmc", args...)
	cmd.Env = systemNDMCEnv(os.Environ())
	return cmd.Output()
}
