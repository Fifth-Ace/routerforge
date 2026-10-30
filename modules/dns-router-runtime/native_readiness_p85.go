package dnsrouter

// P8.5: a passive view of native bindings and the runtime capability.
// A matching ndnproxy entry is NOT proof of a reachable policy, working DNS,
// encrypted egress, or permission to mutate Keenetic settings.

import (
	"net"
)

type NativeBridgeReadinessP85 struct {
	Mode                      string                    `json:"mode"`
	ReadOnly                  bool                      `json:"read_only"`
	CapabilityEnabled         bool                      `json:"capability_enabled"`
	ShadowActive              bool                      `json:"shadow_active"`
	Listener                  string                    `json:"listener"`
	InventoryAvailable        bool                      `json:"inventory_available"`
	PolicyEgressVerified      bool                      `json:"policy_egress_verified"`
	TransportEncryptionProven bool                      `json:"transport_encryption_proven"`
	ActivationAllowed         bool                      `json:"activation_allowed"`
	ChangesApplied            bool                      `json:"changes_applied"`
	Bindings                  []NativeBindingPreviewP83 `json:"bindings"`
}

func BuildNativeBridgeReadinessP85(cfg Config, inventory NativeDNSInventoryP82, capability, active bool, listener string) NativeBridgeReadinessP85 {
	preview := BuildNativeBridgePreviewP83(cfg, inventory)
	out := NativeBridgeReadinessP85{
		Mode: "read-only-native-bridge-status", ReadOnly: true,
		CapabilityEnabled: capability, ShadowActive: active,
		Listener: listener, InventoryAvailable: inventory.Available,
		Bindings: preview.Bindings,
	}
	// A shadow listener configured outside the saved listener object must also
	// be excluded from candidate local ports. Runtime always revalidates.
	if listener != "" {
		_, listenPort, listenErr := net.SplitHostPort(listener)
		if listenErr != nil {
			for i := range out.Bindings {
				out.Bindings[i].Ready = false
				out.Bindings[i].Address = ""
				out.Bindings[i].Reason = "invalid shadow listener"
			}
			return out
		}
		for i := range out.Bindings {
			b := &out.Bindings[i]
			if !b.Ready {
				continue
			}
			_, port, err := net.SplitHostPort(b.Address)
			if err != nil || port == listenPort {
				b.Ready = false
				b.Address = ""
				b.Reason = "native endpoint conflicts with shadow listener"
			}
		}
	}
	return out
}
