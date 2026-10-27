package dnsrouter

// Read-only local capability inventory. No ndmc execution, network probes,
// port binding, config writes or firewall inspection beyond proc socket lists.
import (
	"bufio"
	"os"
	"os/exec"
	"strings"
)

type KeeneticDiscovery struct {
	Mode                     string `json:"mode"`
	NDMCExecutableAvailable  bool   `json:"ndmc_executable_available"`
	ProcSocketTableAvailable bool   `json:"proc_socket_table_available"`
	Port53Entries            int    `json:"port53_entries"`
	OptAvailable             bool   `json:"entware_opt_available"`
	DNSOverrideState         string `json:"dns_override_state"`
	DNSProxyCompatibility    string `json:"dns_proxy_compatibility"`
	ChangesApplied           bool   `json:"changes_applied"`
}

func ReadOnlyKeeneticDiscovery() KeeneticDiscovery {
	result := KeeneticDiscovery{Mode: "read-only", DNSProxyCompatibility: "not_verified_on_hardware", ChangesApplied: false, DNSOverrideState: "not_inspected"}
	_, err := exec.LookPath("ndmc")
	result.NDMCExecutableAvailable = err == nil
	if st, err := os.Stat("/opt"); err == nil && st.IsDir() {
		result.OptAvailable = true
	}
	// We deliberately cannot determine whether dns-override is active from a
	// package marker alone. Report uncertainty rather than claiming disabled.
	// The only measured datum below is an unprivileged count of socket table
	// entries bound to port 53. It is NOT proof of DNS Proxy ownership.
	for _, p := range []string{"/proc/net/udp", "/proc/net/udp6", "/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		result.ProcSocketTableAvailable = true
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 4 {
				continue
			}
			part := strings.Split(fields[1], ":")
			if len(part) == 2 && strings.EqualFold(part[1], "0035") {
				// TCP: 0A is LISTEN; UDP: 07 is normally bound.
				if strings.Contains(p, "udp") || fields[3] == "0A" {
					result.Port53Entries++
				}
			}
		}
		f.Close()
	}
	return result
}
