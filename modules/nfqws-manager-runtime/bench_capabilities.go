package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

const (
	benchCommandTimeout = 3 * time.Second
	benchOutputMax      = 256 << 10
	benchQueueMin       = 30000
	benchQueueMax       = 30127
)

type benchProcessInfo struct {
	PID          int    `json:"pid"`
	Executable   string `json:"executable,omitempty"`
	QueueNumbers []int  `json:"queue_numbers"`
}

type benchCapabilities struct {
	ReadOnly               bool               `json:"read_only"`
	BenchEnabled            bool               `json:"bench_enabled"`
	SafeToBench             bool               `json:"safe_to_bench"`
	CandidateBinary         string             `json:"candidate_binary,omitempty"`
	CandidateSpawnCapable   bool               `json:"candidate_spawn_capable"`
	IPTablesPath            string             `json:"iptables_path,omitempty"`
	IPTablesSavePath        string             `json:"iptables_save_path,omitempty"`
	NFTPath                 string             `json:"nft_path,omitempty"`
	FirewallBackend         string             `json:"firewall_backend,omitempty"`
	FirewallInventoryOK     bool               `json:"firewall_inventory_ok"`
	KernelQueueInventoryOK  bool               `json:"kernel_queue_inventory_ok"`
	QueueInventoryComplete  bool               `json:"queue_inventory_complete"`
	OccupiedQueues          []int              `json:"occupied_queues"`
	RecommendedQueue        int                `json:"recommended_queue,omitempty"`
	ActiveNFQWS2            []benchProcessInfo `json:"active_nfqws2"`
	CleanupProofImplemented bool               `json:"cleanup_proof_implemented"`
	Blockers                []string           `json:"blockers"`
	Warnings                []string           `json:"warnings"`
}

func registerBenchCapabilityRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/bench-capabilities", getOnly(handleBenchCapabilities))
}

func handleBenchCapabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, readBenchCapabilities())
}

var (
	benchQNumPattern       = regexp.MustCompile(`(?i)(?:--qnum|--queue-num)(?:=|\s+)(\d{1,5})`)
	benchQBalancePattern   = regexp.MustCompile(`(?i)--queue-balance(?:=|\s+)(\d{1,5}):(\d{1,5})`)
	benchNFTQueuePattern   = regexp.MustCompile(`(?i)\bqueue\s+num\s+(\d{1,5})(?:-(\d{1,5}))?`)
)

func appendQueueRange(set map[int]bool, start, end int) {
	if start < 0 || end < 0 || start > 65535 || end > 65535 || end < start {
		return
	}
	if end-start > 255 {
		return
	}
	for q := start; q <= end; q++ {
		set[q] = true
	}
}

func parseBenchQueueNumbers(text string) []int {
	set := map[int]bool{}
	for _, m := range benchQNumPattern.FindAllStringSubmatch(text, -1) {
		if q, err := strconv.Atoi(m[1]); err == nil && q >= 0 && q <= 65535 {
			set[q] = true
		}
	}
	for _, m := range benchQBalancePattern.FindAllStringSubmatch(text, -1) {
		a, e1 := strconv.Atoi(m[1])
		b, e2 := strconv.Atoi(m[2])
		if e1 == nil && e2 == nil {
			appendQueueRange(set, a, b)
		}
	}
	for _, m := range benchNFTQueuePattern.FindAllStringSubmatch(text, -1) {
		a, e1 := strconv.Atoi(m[1])
		if e1 != nil {
			continue
		}
		b := a
		if m[2] != "" {
			if parsed, err := strconv.Atoi(m[2]); err == nil {
				b = parsed
			}
		}
		appendQueueRange(set, a, b)
	}
	out := make([]int, 0, len(set))
	for q := range set {
		out = append(out, q)
	}
	sort.Ints(out)
	return out
}

func parseNFNetlinkQueue(text string) []int {
	set := map[int]bool{}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		q, err := strconv.Atoi(fields[0])
		if err == nil && q >= 0 && q <= 65535 {
			set[q] = true
		}
	}
	out := make([]int, 0, len(set))
	for q := range set {
		out = append(out, q)
	}
	sort.Ints(out)
	return out
}

func mergeBenchQueues(groups ...[]int) []int {
	set := map[int]bool{}
	for _, group := range groups {
		for _, q := range group {
			if q >= 0 && q <= 65535 {
				set[q] = true
			}
		}
	}
	out := make([]int, 0, len(set))
	for q := range set {
		out = append(out, q)
	}
	sort.Ints(out)
	return out
}

func firstFreeBenchQueue(occupied []int) int {
	used := map[int]bool{}
	for _, q := range occupied {
		used[q] = true
	}
	for q := benchQueueMin; q <= benchQueueMax; q++ {
		if !used[q] {
			return q
		}
	}
	return 0
}

func findExecutable(names ...string) string {
	for _, name := range names {
		if path, err := exec.LookPath(name); err == nil && executableFile(path) {
			return path
		}
	}
	return ""
}

func readBenchProcesses() ([]benchProcessInfo, []int) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return []benchProcessInfo{}, []int{}
	}
	processes := []benchProcessInfo{}
	allQueues := []int{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		cmdRaw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		cmdline := strings.TrimSpace(strings.ReplaceAll(string(cmdRaw), "\x00", " "))
		if !strings.Contains(strings.ToLower(cmdline), "nfqws2") {
			continue
		}
		exePath, _ := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		queues := parseBenchQueueNumbers(cmdline)
		allQueues = append(allQueues, queues...)
		processes = append(processes, benchProcessInfo{
			PID: pid, Executable: exePath, QueueNumbers: queues,
		})
	}
	sort.Slice(processes, func(i, j int) bool { return processes[i].PID < processes[j].PID })
	return processes, mergeBenchQueues(allQueues)
}

func readFirewallQueueInventory(iptablesSavePath, nftPath string) ([]int, string, bool, string) {
	queues := []int{}
	sources := []string{}
	warning := ""
	if iptablesSavePath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), benchCommandTimeout)
		output, err := safety.RunCommand(ctx, benchOutputMax, iptablesSavePath)
		cancel()
		if err == nil && ctx.Err() == nil {
			queues = append(queues, parseBenchQueueNumbers(string(output))...)
			sources = append(sources, "iptables-save")
		} else {
			warning = "iptables-save inventory failed"
		}
	}
	if nftPath != "" {
		ctx, cancel := context.WithTimeout(context.Background(), benchCommandTimeout)
		output, err := safety.RunCommand(ctx, benchOutputMax, nftPath, "list", "ruleset")
		cancel()
		if err == nil && ctx.Err() == nil {
			queues = append(queues, parseBenchQueueNumbers(string(output))...)
			sources = append(sources, "nft")
		} else if warning == "" {
			warning = "nft ruleset inventory failed"
		}
	}
	return mergeBenchQueues(queues), strings.Join(sources, "+"), len(sources) > 0, warning
}

func readKernelQueueInventory() ([]int, bool) {
	data, err := os.ReadFile("/proc/net/netfilter/nfnetlink_queue")
	if err != nil {
		return []int{}, false
	}
	return parseNFNetlinkQueue(string(data)), true
}

func readBenchCapabilities() benchCapabilities {
	result := benchCapabilities{
		ReadOnly:               true,
		BenchEnabled:            false,
		SafeToBench:             false,
		ActiveNFQWS2:            []benchProcessInfo{},
		OccupiedQueues:          []int{},
		Blockers:                []string{},
		Warnings:                []string{},
		CleanupProofImplemented: false,
	}

	result.IPTablesPath = findExecutable("iptables")
	result.IPTablesSavePath = findExecutable("iptables-save")
	result.NFTPath = findExecutable("nft")

	processes, processQueues := readBenchProcesses()
	result.ActiveNFQWS2 = processes

	for _, proc := range processes {
		if result.CandidateBinary == "" && proc.Executable != "" && executableFile(proc.Executable) {
			result.CandidateBinary = proc.Executable
		}
	}
	if result.CandidateBinary == "" {
		result.CandidateBinary = findExecutable("nfqws2", "nfqws")
	}
	result.CandidateSpawnCapable = result.CandidateBinary != ""

	firewallQueues, backend, firewallOK, warning := readFirewallQueueInventory(result.IPTablesSavePath, result.NFTPath)
	result.FirewallBackend = backend
	result.FirewallInventoryOK = firewallOK
	if warning != "" {
		result.Warnings = append(result.Warnings, warning)
	}

	kernelQueues, kernelOK := readKernelQueueInventory()
	result.KernelQueueInventoryOK = kernelOK

	result.OccupiedQueues = mergeBenchQueues(processQueues, firewallQueues, kernelQueues)
	result.QueueInventoryComplete = firewallOK && kernelOK
	if result.QueueInventoryComplete {
		result.RecommendedQueue = firstFreeBenchQueue(result.OccupiedQueues)
	}

	if !result.CandidateSpawnCapable {
		result.Blockers = append(result.Blockers, "candidate nfqws2 executable was not proven")
	}
	if !result.FirewallInventoryOK {
		result.Blockers = append(result.Blockers, "firewall NFQUEUE inventory is not proven")
	}
	if !result.KernelQueueInventoryOK {
		result.Blockers = append(result.Blockers, "kernel NFQUEUE binding inventory is not proven")
	}
	if result.RecommendedQueue == 0 && result.QueueInventoryComplete {
		result.Blockers = append(result.Blockers, "no free queue found in RouterForge reserved bench range")
	}
	result.Blockers = append(result.Blockers, "isolated bench lifecycle and cleanup proof are not implemented yet")

	if len(result.ActiveNFQWS2) == 0 {
		result.Warnings = append(result.Warnings, "active nfqws2 process was not detected")
	}

	return result
}
