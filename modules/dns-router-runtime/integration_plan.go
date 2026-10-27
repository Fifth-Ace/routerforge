package dnsrouter

// DPR2-F is evidence collection and an integration DESIGN only. No code here
// changes system DNS, Keenetic settings, firewall, routes or listening ports.

type KeeneticIntegrationCandidate struct {
	ID             string   `json:"id"`
	Purpose        string   `json:"purpose"`
	Classification string   `json:"classification"`
	EvidenceGates  []string `json:"evidence_gates"`
	Hazards        []string `json:"hazards"`
}

type KeeneticIntegrationPlan struct {
	Mode              string                         `json:"mode"`
	ActivationAllowed bool                           `json:"activation_allowed"`
	HardwareAccepted  bool                           `json:"hardware_accepted"`
	Decision          string                         `json:"decision"`
	Prerequisites     []string                       `json:"prerequisites"`
	Observed          []string                       `json:"observed"`
	Unknowns          []string                       `json:"unknowns"`
	Candidates        []KeeneticIntegrationCandidate `json:"candidates"`
	RollbackContract  []string                       `json:"rollback_contract"`
	ChangesApplied    bool                           `json:"changes_applied"`
}

func BuildKeeneticIntegrationPlan(d KeeneticDiscovery) KeeneticIntegrationPlan {
	p := KeeneticIntegrationPlan{
		Mode: "read-only-design", ActivationAllowed: false, HardwareAccepted: false,
		Decision: "NO_ACTIVATION_BEFORE_EXPLICIT_HARDWARE_ACCEPTANCE",
		Prerequisites: []string{
			"record real KeeneticOS version, topology and physical architecture",
			"prove which service owns port 53 and whether dns-override is enabled",
			"prove local suffix resolution and no DNS forwarding loop",
			"prove UDP and TCP to a loopback high-port listener on actual device",
			"prove clean rollback after power-loss and upstream timeout scenarios",
			"obtain explicit user confirmation before mutating Keenetic DNS settings",
		},
		Unknowns: []string{
			"Keenetic DNS Proxy selection semantics for a global IP:port upstream on this firmware",
			"Keenetic built-in DNS filtering, encrypted upstream and priority interactions",
			"locally registered names, reverse DNS and split-horizon behavior",
			"dns-override state and active resolver ownership",
			"IPv4/IPv6 behavior under DNS Proxy and clients with hardcoded resolvers",
		},
		Candidates: []KeeneticIntegrationCandidate{
			{ID: "manual-shadow", Purpose: "offline verification using 127.0.0.1 and a high port", Classification: "AVAILABLE_NOW_WITH_EXPLICIT_START", EvidenceGates: []string{"manual localhost UDP/TCP queries pass", "no system DNS change"}},
			{ID: "keenetic-dns-proxy-upstream", Purpose: "Keenetic DNS Proxy conditionally sends selected DNS questions to DNS Router high port", Classification: "RESEARCH_ONLY", EvidenceGates: []string{"exact firmware CLI/RCI support confirmed", "local DNS preservation proven", "upstream recursion impossible", "device rollback demonstrated"}, Hazards: []string{"global forwarding can cause recursive DNS loops", "firmware may not preserve split DNS selection"}},
			{ID: "client-opt-in", Purpose: "explicit client or test container DNS to router high port", Classification: "RESEARCH_ONLY", EvidenceGates: []string{"explicit bind/interface ACL", "IPv4/IPv6 firewall behavior reviewed", "no exposure outside intended clients"}},
			{ID: "dns-override-or-redirection", Purpose: "replace/redirect Keenetic's DNS path", Classification: "NOT_APPROVED", EvidenceGates: []string{"separate explicit architecture decision", "full preimage backup and unattended rollback", "offline recovery acceptance"}, Hazards: []string{"may break KeeneticOS local-name resolution", "may intercept management and bootstrapping DNS", "may affect user-installed DNS projects"}},
		},
		RollbackContract: []string{
			"snapshot exact existing Keenetic DNS configuration before any future mutation",
			"stage settings with immutable identity and a bounded rollback watchdog",
			"verify local names, external A/AAAA, TCP/UDP, DoT/DoH and IPv6",
			"restore exact preimage and verify restored DNS if any gate fails",
			"report AMBIGUOUS instead of PASS if rollback cannot be proven",
		},
		ChangesApplied: false,
	}
	if d.NDMCExecutableAvailable {
		p.Observed = append(p.Observed, "ndmc executable exists (not executed)")
	} else {
		p.Observed = append(p.Observed, "ndmc executable not found in process PATH")
	}
	if d.ProcSocketTableAvailable {
		p.Observed = append(p.Observed, "kernel socket table readable; port 53 entry count is observational only")
	} else {
		p.Observed = append(p.Observed, "kernel socket table unavailable")
	}
	if d.OptAvailable {
		p.Observed = append(p.Observed, "/opt directory exists")
	} else {
		p.Observed = append(p.Observed, "/opt directory missing or inaccessible")
	}
	return p
}
