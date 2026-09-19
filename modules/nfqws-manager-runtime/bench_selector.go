package main

const (
	benchProbeMark     = uint32(0x40000000)
	benchProbeMarkMask = uint32(0x40000000)
	benchProbeProtocol = "tcp"
	benchProbePort     = 443
)

type benchSelectorHook struct {
	Table     string `json:"table"`
	Chain     string `json:"chain"`
	Relation  string `json:"relation"`
	Anchor    string `json:"anchor"`
	Direction string `json:"direction"`
}

type benchSelectorContract struct {
	Implemented           bool                `json:"implemented"`
	MutationImplemented   bool                `json:"mutation_implemented"`
	IPv4Only              bool                `json:"ipv4_only"`
	Protocol              string              `json:"protocol"`
	RemotePort            int                 `json:"remote_port"`
	DestinationExact      bool                `json:"destination_exact"`
	LocalPortExact        bool                `json:"local_port_exact"`
	LocalPortAllocation   string              `json:"local_port_allocation"`
	PacketMark            uint32              `json:"packet_mark"`
	PacketMarkMask        uint32              `json:"packet_mark_mask"`
	ConnmarkRequired      bool                `json:"connmark_required"`
	OwnerMatchRequired    bool                `json:"owner_match_required"`
	RawTableRequired      bool                `json:"raw_table_required"`
	ProductionQueueBypass bool                `json:"production_queue_bypass"`
	MarkClearedAfterHook  bool                `json:"mark_cleared_after_hook"`
	SingleSessionOnly     bool                `json:"single_session_only"`
	RuleIdentity          string              `json:"rule_identity"`
	Hooks                 []benchSelectorHook `json:"hooks"`
}

func buildBenchSelectorContract() benchSelectorContract {
	return benchSelectorContract{
		Implemented:           true,
		MutationImplemented:   true,
		IPv4Only:              true,
		Protocol:              benchProbeProtocol,
		RemotePort:            benchProbePort,
		DestinationExact:      true,
		LocalPortExact:        true,
		LocalPortAllocation:   "dynamic exact port bound by RouterForge session",
		PacketMark:            benchProbeMark,
		PacketMarkMask:        benchProbeMarkMask,
		ConnmarkRequired:      false,
		OwnerMatchRequired:    false,
		RawTableRequired:      false,
		ProductionQueueBypass: true,
		MarkClearedAfterHook:  true,
		SingleSessionOnly:     true,
		RuleIdentity:          "routerforge-bench:<session-id>",
		Hooks: []benchSelectorHook{
			{
				Table:     "mangle",
				Chain:     "POSTROUTING",
				Relation:  "before",
				Anchor:    "nfqws_post",
				Direction: "outbound exact destination:443 + exact local source port -> mark -> reserved NFQUEUE",
			},
			{
				Table:     "mangle",
				Chain:     "POSTROUTING",
				Relation:  "after",
				Anchor:    "nfqws_post",
				Direction: "outbound exact tuple -> clear RouterForge packet mark",
			},
			{
				Table:     "mangle",
				Chain:     "PREROUTING",
				Relation:  "before",
				Anchor:    "nfqws_pre",
				Direction: "inbound exact source:443 + exact local destination port -> mark -> reserved NFQUEUE",
			},
			{
				Table:     "mangle",
				Chain:     "PREROUTING",
				Relation:  "after",
				Anchor:    "nfqws_pre",
				Direction: "inbound exact reverse tuple -> clear RouterForge packet mark",
			},
		},
	}
}
