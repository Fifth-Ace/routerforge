package main

// vNext optional modules register here without widening the default mutation
// surface in module_proxy.go. The generic proxy remains GET/HEAD-only for
// these IDs until a later Snapshot/Transaction consumer gate explicitly
// authorizes a bounded mutation API.
func init() {
	moduleSockets["maintenance"] = []string{"/opt/var/run/routerforge-maintenance.sock"}
	moduleSockets["network-tools"] = []string{"/opt/var/run/routerforge-network-tools.sock"}
	moduleSockets["integrations"] = []string{"/opt/var/run/routerforge-integrations.sock"}
	moduleSockets["developer-tools"] = []string{"/opt/var/run/routerforge-developer-tools.sock"}

	modulePackageNames["maintenance"] = "routerforge-maintenance"
	modulePackageNames["network-tools"] = "routerforge-network-tools"
	modulePackageNames["integrations"] = "routerforge-integrations"
	modulePackageNames["developer-tools"] = "routerforge-developer-tools"
}
