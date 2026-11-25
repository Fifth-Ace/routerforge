package main

import (
	"context"
	"github.com/Fifth-Ace/routerforge/internal/platform/contract"
	"net/http"
	"strings"
	"time"
)

func registerFoundationConsumers(mux *http.ServeMux, s *contract.Store) {
	registerFoundationConsumersWithReader(mux, s, foundationReadNative)
}
func registerFoundationConsumersWithReader(mux *http.ServeMux, s *contract.Store, read foundationNativeRead) {
	mux.HandleFunc(foundationPath+"/consumers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowedJSON(w, http.MethodGet)
			return
		}
		q := r.URL.Query()
		consumer, id := q.Get("consumer"), q.Get("id")
		if len(q["consumer"]) != 1 || len(q["id"]) > 1 || len(q["etag"]) > 1 || len(id) > 96 || len(consumer) > 32 {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
			return
		}
		for k := range q {
			if k != "consumer" && k != "id" && k != "etag" {
				writeAuthJSON(w, 400, map[string]string{"error": "INVALID_QUERY"})
				return
			}
		}
		st, e := s.Get()
		if e != nil {
			foundationError(w, e)
			return
		}
		if id == "" {
			g, e := contract.ConsumerGroups(st.Current, consumer)
			if e != nil {
				writeAuthJSON(w, 400, map[string]string{"error": "INVALID_CONSUMER"})
				return
			}
			writeAuthJSON(w, 200, map[string]any{"schemaVersion": 1, "etag": st.Current.ETag, "groups": g})
			return
		}
		if strings.TrimSpace(q.Get("etag")) != st.Current.ETag {
			writeAuthJSON(w, 409, map[string]string{"error": "REVISION_CONFLICT"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		p, e := contract.ProjectConsumerWithNative(st.Current, consumer, id, foundationNativeResolver(ctx, read))
		if e != nil {
			writeAuthJSON(w, 400, map[string]string{"error": "INVALID_GROUP"})
			return
		}
		writeAuthJSON(w, 200, p)
	})
}
