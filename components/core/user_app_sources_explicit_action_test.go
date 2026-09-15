package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func stubExplicitSourceResolver(t *testing.T, fingerprint string, called *int) {
	t.Helper()
	old := appSourceResolve
	appSourceResolve = func(context.Context, string, string) (appSourceCache, error) {
		if called != nil {
			*called = *called + 1
		}
		return appSourceCache{
			SchemaVersion:  appSourcesSchemaVersion,
			Kind:           "app",
			RegistryID:     "single",
			Name:           "Demo",
			ResolvedURL:    "https://example.com/routerforge.json",
			ManifestSHA256: fingerprint,
			Entries: []catalogItem{{
				ID:        "demo",
				Kind:      "integration",
				Name:      "Demo",
				Publisher: catalogPublisher{Name: "Example"},
			}},
		}, nil
	}
	t.Cleanup(func() { appSourceResolve = old })
}

func TestAppSourceAddRequiresExplicitPreviewFingerprint(t *testing.T) {
	withTempAppSources(t)

	fingerprint := strings.Repeat("a", 64)
	calls := 0
	stubExplicitSourceResolver(t, fingerprint, &calls)

	baseRequest := appSourceMutationRequest{
		URL:  "https://example.com/routerforge.json",
		Kind: "app",
	}

	if _, _, err := addAppSource(context.Background(), baseRequest); err == nil ||
		!strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("missing explicit add confirmation was not rejected: %v", err)
	}
	if calls != 0 {
		t.Fatalf("resolver called before explicit confirmation: %d", calls)
	}

	request := baseRequest
	request.Confirm = appSourceAddConfirm
	if _, _, err := addAppSource(context.Background(), request); err == nil ||
		!strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("missing preview fingerprint was not rejected: %v", err)
	}
	if calls != 0 {
		t.Fatalf("resolver called before valid fingerprint: %d", calls)
	}

	request.Fingerprint = strings.Repeat("b", 64)
	if _, _, err := addAppSource(context.Background(), request); err == nil ||
		!strings.Contains(err.Error(), "changed since preview") {
		t.Fatalf("changed source fingerprint was not rejected: %v", err)
	}
	if calls != 1 {
		t.Fatalf("resolver calls after mismatch=%d, want 1", calls)
	}

	if _, err := os.Stat(appSourcesConfigPath); !os.IsNotExist(err) {
		t.Fatalf("mismatched preview persisted config: %v", err)
	}

	request.Fingerprint = fingerprint
	source, preview, err := addAppSource(context.Background(), request)
	if err != nil {
		t.Fatalf("confirmed add failed: %v", err)
	}
	if source.ID == "" || preview.Fingerprint != fingerprint {
		t.Fatalf("unexpected confirmed add result: source=%#v preview=%#v", source, preview)
	}
	if calls != 2 {
		t.Fatalf("resolver calls after successful add=%d, want 2", calls)
	}
}

func TestAppSourcePreviewDoesNotPersistOrEnableSource(t *testing.T) {
	withTempAppSources(t)

	fingerprint := strings.Repeat("c", 64)
	calls := 0
	stubExplicitSourceResolver(t, fingerprint, &calls)

	body := `{"url":"https://example.com/routerforge.json","kind":"app"}`
	req := httptest.NewRequest(http.MethodPost, "http://router.local/api/apps/sources/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://router.local")
	rec := httptest.NewRecorder()

	handleAppSourcePreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("preview resolver calls=%d, want 1", calls)
	}
	if _, err := os.Stat(appSourcesConfigPath); !os.IsNotExist(err) {
		t.Fatalf("preview persisted source config: %v", err)
	}
	if _, err := os.Stat(appSourcesCacheDir); !os.IsNotExist(err) {
		t.Fatalf("preview persisted source cache: %v", err)
	}
}

func TestAppSourceFingerprintValidationIsCanonical(t *testing.T) {
	good := strings.Repeat("a", 64)
	if !validAppSourceFingerprint(good) {
		t.Fatal("canonical fingerprint rejected")
	}
	for _, value := range []string{
		"",
		strings.Repeat("a", 63),
		strings.Repeat("a", 65),
		strings.Repeat("A", 64),
		strings.Repeat("z", 64),
	} {
		if validAppSourceFingerprint(value) {
			t.Fatalf("invalid fingerprint accepted: %q", value)
		}
	}
}
