package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// v2SelectorWorkerBudget describes admission of isolated live NFQUEUE workers.
// It limits concurrency; it does not reuse memory results instead of live tests.
type v2SelectorWorkerBudget struct {
	Requested    int    `json:"requested"`
	Limit        int    `json:"limit"`
	Effective    int    `json:"effective"`
	TotalMiB     int64  `json:"total_mib,omitempty"`
	AvailableMiB int64  `json:"available_mib,omitempty"`
	Measured     bool   `json:"measured"`
	Capped       bool   `json:"capped"`
	Reason       string `json:"reason"`
}

// v2WorkerMemoryValues requires MemAvailable; MemFree alone is not a reliable
// representation of reclaimable memory on a live router.
func v2WorkerMemoryValues(content string) (totalKiB, availableKiB int64, ok bool) {
	if len(content) > 16<<10 {
		return 0, 0, false
	}
	values := make(map[string]int64, 2)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 3 || parts[2] != "kB" {
			continue
		}
		if parts[0] != "MemTotal:" && parts[0] != "MemAvailable:" {
			continue
		}
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n < 0 {
			return 0, 0, false
		}
		values[parts[0]] = n
	}
	if scanner.Err() != nil || values["MemTotal:"] <= 0 {
		return 0, 0, false
	}
	avail, present := values["MemAvailable:"]
	if !present || avail > values["MemTotal:"] {
		return 0, 0, false
	}
	return values["MemTotal:"], avail, true
}

// Limits are deliberately below the upstream nfqws2-ui PHP scanner's worker
// counts: each RouterForge worker has its own nfqws2 process and NFQUEUE rules.
// The thresholds are a conservative admission policy, not measured RSS claims.
func v2WorkerBudgetFromMemory(totalKiB, availableKiB int64, measured bool) v2SelectorWorkerBudget {
	budget := v2SelectorWorkerBudget{Limit: 1, Measured: false, Reason: "MemAvailable недоступен — разрешён один воркер"}
	if !measured || totalKiB <= 0 || availableKiB < 0 || availableKiB > totalKiB {
		return budget
	}
	budget.Measured = true
	budget.TotalMiB = totalKiB / 1024
	budget.AvailableMiB = availableKiB / 1024
	budget.Reason = "лимит по объёму ОЗУ и доступной памяти"
	if availableKiB < 24*1024 {
		budget.Limit = 0
		budget.Reason = "доступно менее 24 МиБ: тестовый воркер не запускаем"
		return budget
	}
	totalCap := v2MaxConcurrency
	switch {
	case totalKiB <= 192*1024:
		totalCap = 1
	case totalKiB <= 320*1024:
		totalCap = 2
	case totalKiB <= 448*1024:
		totalCap = 3
	}
	availableCap := v2MaxConcurrency
	switch {
	case availableKiB < 64*1024:
		availableCap = 1
	case availableKiB < 112*1024:
		availableCap = 2
	case availableKiB < 160*1024:
		availableCap = 3
	}
	if availableCap < totalCap {
		totalCap = availableCap
	}
	budget.Limit = totalCap
	return budget
}

func v2ReadSelectorWorkerBudget() v2SelectorWorkerBudget {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return v2WorkerBudgetFromMemory(0, 0, false)
	}
	total, available, ok := v2WorkerMemoryValues(string(data))
	return v2WorkerBudgetFromMemory(total, available, ok)
}

func v2WorkerBudgetSummary(budget v2SelectorWorkerBudget) string {
	if !budget.Measured {
		return "данные MemAvailable недоступны: лимит один воркер"
	}
	return fmt.Sprintf("ОЗУ %d МиБ, доступно %d МиБ; лимит %d воркеров", budget.TotalMiB, budget.AvailableMiB, budget.Limit)
}
