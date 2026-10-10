package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// natEvidenceHandler observes the current NAT table, never mutates it.
func natEvidenceHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if len(q) != 3 || len(q["wan"]) != 1 || len(q["public"]) != 1 || len(q["target"]) != 1 {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "wan, public and target required"})
		return
	}
	public, e1 := strconv.Atoi(q.Get("public"))
	target, e2 := strconv.Atoi(q.Get("target"))
	wan := q.Get("wan")
	if e1 != nil || e2 != nil || public < 1 || public > 65535 {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid ports"})
		return
	}
	if _, _, err := buildKnockHook(wan, target); err != nil {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid scope"})
		return
	}
	var bin string
	for _, path := range []string{"/opt/sbin/iptables", "/usr/sbin/iptables", "/sbin/iptables"} {
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			bin = path
			break
		}
	}
	if bin == "" {
		jsonReply(w, http.StatusServiceUnavailable, map[string]string{"error": "iptables unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	snapshot, err := safety.RunCommandOutput(ctx, bin, "-t", "nat", "-S")
	if err != nil || len(snapshot) > 1<<20 {
		jsonReply(w, http.StatusServiceUnavailable, map[string]string{"error": "cannot inspect NAT"})
		return
	}
	dest, err := inspectDNATDirect(string(snapshot), wan, public, target)
	source := "direct"
	if err != nil {
		dest, err = inspectDNATNDM(string(snapshot), wan, public, target)
		source = "ndm-one-hop"
	}
	if err != nil {
		jsonReply(w, http.StatusConflict, map[string]any{"module": "port-access-manager", "ready_for_apply": false, "mutation_api": false, "observed_only": true, "error": "matching DNAT not proven"})
		return
	}
	jsonReply(w, http.StatusOK, map[string]any{"module": "port-access-manager", "ready_for_apply": false, "mutation_api": false, "observed_only": true, "wan": wan, "public_port": public, "target_port": target, "destination_ip": dest, "path": source})
}
