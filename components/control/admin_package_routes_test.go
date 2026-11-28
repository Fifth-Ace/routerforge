package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/apptransaction"
)

func TestC7AdminPackageRouteAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, method             string
		auth                     bool
		busy                     bool
		want                     int
		calls, actions, releases int
	}{
		{"read-method", "GET", true, false, 405, 0, 0, 0},
		{"unauthorized", "POST", false, false, 403, 0, 0, 0},
		{"busy", "POST", true, true, 409, 1, 0, 0},
		{"allowed", "POST", true, false, 204, 1, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, actions, releases := 0, 0, 0
			h := adminPackageMutationOnly(func(w http.ResponseWriter, r *http.Request) {
				actions++
				if releases != 0 {
					t.Fatal("lease released before handler")
				}
				w.WriteHeader(204)
			}, func() (func(), error) {
				calls++
				if tc.busy {
					return nil, apptransaction.ErrBusy
				}
				return func() { releases++ }, nil
			})
			r := httptest.NewRequest(tc.method, "/v1/files/write", nil)
			if tc.auth {
				r.Header.Set(adminMutationAuthorizationHeader, adminMutationAuthorizationValue)
			}
			w := httptest.NewRecorder()
			h(w, r)
			if w.Code != tc.want || calls != tc.calls || actions != tc.actions || releases != tc.releases {
				t.Fatal(w.Code, calls, actions, releases)
			}
		})
	}
}
