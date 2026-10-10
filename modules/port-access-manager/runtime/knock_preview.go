package main

import (
	"net/http"
	"strconv"
)

func knockPreviewHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	allowed := map[string]bool{"first": true, "second": true, "third": true, "target": true, "window": true, "ttl": true}
	for key := range q {
		if !allowed[key] || len(q[key]) != 1 {
			jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid query parameters"})
			return
		}
	}
	get := func(key string) (int, error) { return strconv.Atoi(q.Get(key)) }
	var o KnockOptions
	var err error
	for _, item := range []struct {
		key  string
		dest *int
	}{
		{"first", &o.Sequence[0]}, {"second", &o.Sequence[1]}, {"third", &o.Sequence[2]},
		{"target", &o.Target}, {"window", &o.WindowSeconds}, {"ttl", &o.AccessSeconds},
	} {
		*item.dest, err = get(item.key)
		if err != nil {
			jsonReply(w, http.StatusBadRequest, map[string]string{"error": "invalid numeric parameters"})
			return
		}
	}
	plan, err := buildKnockRules(o)
	if err != nil {
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	jsonReply(w, http.StatusOK, plan)
}
