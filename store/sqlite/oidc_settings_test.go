package sqlite_test

import (
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store/test_sqlite"
)

func TestNeedsSetupBeforeAndAfterConfiguring(t *testing.T) {
	dataStore := test_sqlite.New(t)

	if got, want := mustNeedsSetup(t, dataStore), true; got != want {
		t.Fatalf("NeedsSetup()=%v before configuring OIDC, want %v", got, want)
	}

	if err := dataStore.UpdateOIDCSettings(mustOIDCSettings(t)); err != nil {
		t.Fatalf("failed to update OIDC settings: %v", err)
	}

	if got, want := mustNeedsSetup(t, dataStore), false; got != want {
		t.Fatalf("NeedsSetup()=%v after configuring OIDC, want %v", got, want)
	}

	settings, err := dataStore.ReadOIDCSettings()
	if err != nil {
		t.Fatalf("failed to read OIDC settings: %v", err)
	}
	if got, want := settings.IssuerURL.String(), "https://nas.example.com/webman/sso"; got != want {
		t.Errorf("IssuerURL=%s, want %s", got, want)
	}
}

func TestGenerateAndValidateSetupToken(t *testing.T) {
	dataStore := test_sqlite.New(t)

	token, err := dataStore.GenerateSetupToken()
	if err != nil {
		t.Fatalf("failed to generate setup token: %v", err)
	}

	valid, err := dataStore.ValidateSetupToken(token)
	if err != nil {
		t.Fatalf("failed to validate setup token: %v", err)
	}
	if got, want := valid, true; got != want {
		t.Errorf("ValidateSetupToken(correct token)=%v, want %v", got, want)
	}

	wrongToken := picoshare.NewSetupToken()
	valid, err = dataStore.ValidateSetupToken(wrongToken)
	if err != nil {
		t.Fatalf("failed to validate setup token: %v", err)
	}
	if got, want := valid, false; got != want {
		t.Errorf("ValidateSetupToken(wrong token)=%v, want %v", got, want)
	}
}

func TestUpdateOIDCSettingsInvalidatesTheSetupToken(t *testing.T) {
	dataStore := test_sqlite.New(t)

	token, err := dataStore.GenerateSetupToken()
	if err != nil {
		t.Fatalf("failed to generate setup token: %v", err)
	}

	if err := dataStore.UpdateOIDCSettings(mustOIDCSettings(t)); err != nil {
		t.Fatalf("failed to update OIDC settings: %v", err)
	}

	valid, err := dataStore.ValidateSetupToken(token)
	if err != nil {
		t.Fatalf("failed to validate setup token: %v", err)
	}
	if got, want := valid, false; got != want {
		t.Errorf("ValidateSetupToken() after completing setup=%v, want %v", got, want)
	}
}

func TestClearOIDCSettingsRequiresSetupAgain(t *testing.T) {
	dataStore := test_sqlite.New(t)

	if err := dataStore.UpdateOIDCSettings(mustOIDCSettings(t)); err != nil {
		t.Fatalf("failed to update OIDC settings: %v", err)
	}
	if got, want := mustNeedsSetup(t, dataStore), false; got != want {
		t.Fatalf("NeedsSetup()=%v after configuring OIDC, want %v", got, want)
	}

	if err := dataStore.ClearOIDCSettings(); err != nil {
		t.Fatalf("failed to clear OIDC settings: %v", err)
	}

	if got, want := mustNeedsSetup(t, dataStore), true; got != want {
		t.Fatalf("NeedsSetup()=%v after clearing OIDC settings, want %v", got, want)
	}
}

func mustNeedsSetup(t *testing.T, r interface {
	NeedsSetup() (bool, error)
}) bool {
	t.Helper()
	needsSetup, err := r.NeedsSetup()
	if err != nil {
		t.Fatalf("failed to check setup status: %v", err)
	}
	return needsSetup
}

func mustOIDCSettings(t *testing.T) picoshare.OIDCSettings {
	t.Helper()
	issuerURL, err := picoshare.NewOIDCIssuerURL("https://nas.example.com/webman/sso")
	if err != nil {
		t.Fatalf("failed to create issuer URL: %v", err)
	}
	clientID, err := picoshare.NewOIDCClientID("test-client")
	if err != nil {
		t.Fatalf("failed to create client ID: %v", err)
	}
	clientSecret, err := picoshare.NewOIDCClientSecret("test-secret")
	if err != nil {
		t.Fatalf("failed to create client secret: %v", err)
	}
	redirectURL, err := picoshare.NewOIDCRedirectURL("https://picoshare.example.com/oidc/callback")
	if err != nil {
		t.Fatalf("failed to create redirect URL: %v", err)
	}
	return picoshare.OIDCSettings{
		IssuerURL:    issuerURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	}
}
