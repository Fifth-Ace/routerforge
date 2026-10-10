package main

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Fifth-Ace/routerforge/internal/safety"
)

// forwardOrderHandler reports observed filter order only. It never changes firewall state.
func forwardOrderHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if len(q) != 2 || len(q["wan"]) != 1 || len(q["port"]) != 1 {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "wan and port required"})
		return
	}
	port, err := strconv.Atoi(q.Get("port"))
	if err != nil {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid port"})
		return
	}
	wan := q.Get("wan")
	if _, _, err = buildKnockHook(wan, port); err != nil {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid scope"})
		return
	}
	var bin string
	for _, p := range []string{"/opt/sbin/iptables", "/usr/sbin/iptables", "/sbin/iptables"} {
		if st, e := os.Stat(p); e == nil && st.Mode().IsRegular() && st.Mode().Perm()&0111 != 0 {
			bin = p
			break
		}
	}
	if bin == "" {
		jsonReply(w, http.StatusServiceUnavailable, map[string]string{"error": "iptables unavailable"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	snapshot, err := safety.RunCommandOutput(ctx, bin, "-t", "filter", "-S")
	if err != nil || len(snapshot) > 1<<20 {
		jsonReply(w, http.StatusServiceUnavailable, map[string]string{"error": "cannot inspect filter"})
		return
	}
	position, err := checkForwardOrder(string(snapshot), wan, port)
	if err != nil {
		jsonReply(w, http.StatusConflict, map[string]any{"module": "port-access-manager", "ready_for_apply": false, "mutation_api": false, "error": err.Error()})
		return
	}
	jsonReply(w, http.StatusOK, map[string]any{"module": "port-access-manager", "ready_for_apply": false, "mutation_api": false, "wan": wan, "port": port, "planned_position": position, "observed_only": true})
}
