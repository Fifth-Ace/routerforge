package main

import "testing"

func TestP30ADetectionUsesProjectOwnedArtifacts(t *testing.T) {
	tests := []struct {
		name   string
		item   catalogItem
		common []string
		owned  string
	}{
		{
			name:   "keenetic-zapret2-manager",
			item:   keeneticZapret2ManagerCatalogItem(),
			common: []string{"/opt/zapret2", "/opt/lib/opkg"},
			owned:  "/opt/lib/opkg/keenetic_zapret2_manager.sh",
		},
		{
			name:   "wireguard-dpi-bypass",
			item:   wireguardDPIBypassCatalogItem(),
			common: []string{"/opt/etc/ndm/netfilter.d", "/opt/etc/ndm/wan.d"},
			owned:  "/opt/etc/ndm/netfilter.d/wgpass.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commonExists := map[string]bool{}
			for _, path := range tt.common {
				commonExists[path] = true
			}

			item := tt.item
			finalizeCatalogItem(&item, map[string]string{}, map[string]bool{}, func(path string) bool {
				return commonExists[path]
			})
			if item.Installed {
				t.Fatalf("common/shared paths caused false installed state: %#v", item.Detection)
			}

			item = tt.item
			finalizeCatalogItem(&item, map[string]string{}, map[string]bool{}, func(path string) bool {
				return path == tt.owned
			})
			if !item.Installed {
				t.Fatalf("project-owned artifact did not detect installation: %s", tt.owned)
			}
		})
	}
}
