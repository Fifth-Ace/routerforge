package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAntiscanUIRoutesDoNotRedirectIndex(t *testing.T) {
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("<!doctype html><title>antiscan-test</title>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uiDir, "app.js"), []byte("window.antisanTest=true;"), 0644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerUIRoutes(mux, uiDir)

	for _, target := range []string{
		"/v1/ui",
		"/v1/ui/",
		"/v1/ui/index.html?locale=ru&view=overview&rev=test",
	} {
		request := httptest.NewRequest(http.MethodGet, target, nil)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d location=%q body=%q", target, response.Code, response.Header().Get("Location"), response.Body.String())
		}
		if location := response.Header().Get("Location"); location != "" {
			t.Fatalf("%s unexpectedly redirected to %q", target, location)
		}
		if !strings.Contains(response.Body.String(), "antiscan-test") {
			t.Fatalf("%s did not serve index.html: %q", target, response.Body.String())
		}
	}
}

func TestAntiscanUIRoutesServeAssetsWithoutRedirect(t *testing.T) {
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "app.js"), []byte("window.antiscanTest=true;"), 0644); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerUIRoutes(mux, uiDir)

	request := httptest.NewRequest(http.MethodGet, "/v1/ui/app.js", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.String())
	}
	if location := response.Header().Get("Location"); location != "" {
		t.Fatalf("asset unexpectedly redirected to %q", location)
	}
	if !strings.Contains(response.Body.String(), "antiscanTest") {
		t.Fatalf("unexpected asset body %q", response.Body.String())
	}
}

func TestAntiscanUIRoutesRejectEncodedTraversal(t *testing.T) {
	uiDir := t.TempDir()
	mux := http.NewServeMux()
	registerUIRoutes(mux, uiDir)

	request := httptest.NewRequest(http.MethodGet, "/v1/ui/%2e%2e/secret", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d location=%q", response.Code, http.StatusNotFound, response.Header().Get("Location"))
	}
}

func TestAntiscanUIRoutesRejectMutationMethods(t *testing.T) {
	uiDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(uiDir, "index.html"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	registerUIRoutes(mux, uiDir)

	request := httptest.NewRequest(http.MethodPost, "/v1/ui/index.html", strings.NewReader("nope"))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d", response.Code, http.StatusNotFound)
	}
}
