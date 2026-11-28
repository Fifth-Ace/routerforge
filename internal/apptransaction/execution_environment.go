package apptransaction

import (
	"errors"
	"strings"
)

const leaseEnv = "RF_APP_TRANSACTION_LEASE_FD"
const profilingIntentEnv = "RF_APP_TRANSACTION_PROFILING_INTENT"

// Only trusted local package commands populate these values. Strip inherited
// copies before setting the actual lease descriptor and optional intent.
func executionEnvironment(base []string, intent string) ([]string, error) {
	if intent != "" && intent != "on" && intent != "off" {
		return nil, errors.New("invalid profiling migration intent")
	}
	env := make([]string, 0, len(base)+2)
	for _, v := range base {
		key, _, _ := strings.Cut(v, "=")
		if key != leaseEnv && key != profilingIntentEnv {
			env = append(env, v)
		}
	}
	env = append(env, leaseEnv+"=3")
	if intent != "" {
		env = append(env, profilingIntentEnv+"="+intent)
	}
	return env, nil
}
