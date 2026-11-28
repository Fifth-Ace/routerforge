//go:build !linux

package main

import "net/http"

func initCoreAppTransactions()                    {}
func acquireCorePackageMutation() (func(), error) { return func() {}, nil }
func coreMutationHTTPStatus(error) int            { return http.StatusServiceUnavailable }
func registerCoreAppTransactionHandlers(mux *http.ServeMux, auth *authManager) {
	unavailable := func(w http.ResponseWriter, r *http.Request) {
		writeCatalogJSON(w, http.StatusNotImplemented, map[string]any{"execution_available": false, "error": "transaction integration unavailable on this platform"})
	}
	mux.HandleFunc("/api/apps/transactions", unavailable)
	mux.HandleFunc("/api/apps/transactions/preview", unavailable)
}

func serveCoreHTTPWithEvidence(server *http.Server, _ string) error { return server.ListenAndServe() }
