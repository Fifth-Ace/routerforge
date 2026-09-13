package main

// Network Tools is the only post-0.7 optional top-level module kept on the
// development train. The generic module proxy remains GET/HEAD-only for this
// module; active diagnostics do not expose configuration mutation endpoints.
func init() {
	moduleSockets["network-tools"] = []string{"/opt/var/run/routerforge-network-tools.sock"}
	modulePackageNames["network-tools"] = "routerforge-network-tools"
}
