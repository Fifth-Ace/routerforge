package main

// During an actual DNS Router update, let opkg execute its normal upgrade
// lifecycle (prerm upgrade -> maintenance -> postinst -> Resume). Forced
// reinstallation may use remove semantics, which explicitly commits OFF.
func routerForgeIPKOpkgArgs(action, pkg, local string) []string {
	if action == "update" && pkg != "routerforge-dns-router" {
		return []string{"--force-reinstall", "install", local}
	}
	return []string{"install", local}
}
