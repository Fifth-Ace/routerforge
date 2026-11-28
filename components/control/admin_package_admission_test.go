package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Fifth-Ace/routerforge/internal/apptransaction"
)

func TestAdminPackageAdmissionDenial(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{apptransaction.ErrBusy, http.StatusConflict}, {apptransaction.ErrRecovery, http.StatusConflict}, {errors.New("unavailable"), http.StatusServiceUnavailable}} {
		w := httptest.NewRecorder()
		release, ok := adminPackageMutationAdmission(w, func() (func(), error) { return nil, tc.err })
		if ok || release != nil || w.Code != tc.status {
			t.Fatalf("admission: %v %d", ok, w.Code)
		}
	}
	w := httptest.NewRecorder()
	closed := false
	release, ok := adminPackageMutationAdmission(w, func() (func(), error) { return func() { closed = true }, nil })
	if !ok || release == nil {
		t.Fatal("free admission rejected")
	}
	release()
	if !closed {
		t.Fatal("lease not released")
	}
}
