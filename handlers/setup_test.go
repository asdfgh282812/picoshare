package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtlynch/picoshare/handlers"
	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

// TestAdminOIDCSettingsPutBlankSecretKeepsExisting verifies that an
// administrator can update other OIDC settings without resupplying the
// client secret, since the settings page never echoes it back to them.
func TestAdminOIDCSettingsPutBlankSecretKeepsExisting(t *testing.T) {
	dataStore := test_sqlite.New(t)
	_, adminCookie := mustLoginAsUser(t, &dataStore, "admin", mustParseTime("2024-01-01T00:00:00Z"))

	if err := dataStore.UpdateOIDCSettings(picoshare.OIDCSettings{
		IssuerURL:    mustOIDCIssuerURL(t, "https://idp.example.com"),
		ClientID:     mustOIDCClientID(t, "original-client-id"),
		ClientSecret: mustOIDCClientSecret(t, "original-secret"),
		RedirectURL:  mustOIDCRedirectURL(t, "https://picoshare.example.com/oidc/callback"),
	}); err != nil {
		t.Fatalf("failed to seed OIDC settings: %v", err)
	}

	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	payload := `{
		"issuerUrl": "https://idp.example.com",
		"clientId": "updated-client-id",
		"clientSecret": "",
		"redirectUrl": "https://picoshare.example.com/oidc/callback"
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/oidc-settings", strings.NewReader(payload))
	req.Header.Add("Content-Type", "text/json")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusOK; got != want {
		t.Fatalf("PUT /api/admin/oidc-settings returned status %d, want %d", got, want)
	}

	settings, err := dataStore.ReadOIDCSettings()
	if err != nil {
		t.Fatalf("failed to read back OIDC settings: %v", err)
	}
	if got, want := settings.ClientID.String(), "updated-client-id"; got != want {
		t.Errorf("client ID=%q, want %q", got, want)
	}
	if got, want := settings.ClientSecret.String(), "original-secret"; got != want {
		t.Errorf("client secret=%q, want %q (blank submission should keep the existing secret)", got, want)
	}
}

// TestAdminOIDCSettingsPutRejectsBlankSecretWithNoneConfigured verifies that
// leaving the secret blank fails clearly when there's no existing secret to
// fall back to, instead of silently saving an empty one.
func TestAdminOIDCSettingsPutRejectsBlankSecretWithNoneConfigured(t *testing.T) {
	dataStore := test_sqlite.New(t)
	_, adminCookie := mustLoginAsUser(t, &dataStore, "admin", mustParseTime("2024-01-01T00:00:00Z"))
	s := handlers.New(handlers.Params{Store: &dataStore, CheckSpace: nilSpaceCheckFunc, Collector: nilGarbageCollector, Now: time.Now})

	payload := `{
		"issuerUrl": "https://idp.example.com",
		"clientId": "some-client-id",
		"clientSecret": "",
		"redirectUrl": "https://picoshare.example.com/oidc/callback"
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/admin/oidc-settings", strings.NewReader(payload))
	req.Header.Add("Content-Type", "text/json")
	req.AddCookie(adminCookie)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	res := rec.Result()

	if got, want := res.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("PUT /api/admin/oidc-settings with no prior secret returned status %d, want %d", got, want)
	}
}

func mustOIDCIssuerURL(t *testing.T, raw string) picoshare.OIDCIssuerURL {
	t.Helper()
	v, err := picoshare.NewOIDCIssuerURL(raw)
	if err != nil {
		t.Fatalf("failed to create OIDC issuer URL: %v", err)
	}
	return v
}

func mustOIDCClientID(t *testing.T, raw string) picoshare.OIDCClientID {
	t.Helper()
	v, err := picoshare.NewOIDCClientID(raw)
	if err != nil {
		t.Fatalf("failed to create OIDC client ID: %v", err)
	}
	return v
}

func mustOIDCClientSecret(t *testing.T, raw string) picoshare.OIDCClientSecret {
	t.Helper()
	v, err := picoshare.NewOIDCClientSecret(raw)
	if err != nil {
		t.Fatalf("failed to create OIDC client secret: %v", err)
	}
	return v
}

func mustOIDCRedirectURL(t *testing.T, raw string) picoshare.OIDCRedirectURL {
	t.Helper()
	v, err := picoshare.NewOIDCRedirectURL(raw)
	if err != nil {
		t.Fatalf("failed to create OIDC redirect URL: %v", err)
	}
	return v
}
