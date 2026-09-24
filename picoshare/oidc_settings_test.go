package picoshare_test

import (
	"testing"

	"github.com/mtlynch/picoshare/picoshare"
)

func TestNewOIDCIssuerURL(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "a typical https URL is valid",
			input:           "https://nas.example.com/webman/sso",
			isValidExpected: true,
		},
		{
			explanation:     "an http URL is valid",
			input:           "http://nas.example.com/webman/sso",
			isValidExpected: true,
		},
		{
			explanation:     "a relative URL is invalid",
			input:           "/webman/sso",
			isValidExpected: false,
		},
		{
			explanation:     "empty text is invalid",
			input:           "",
			isValidExpected: false,
		},
		{
			explanation:     "a non-http(s) scheme is invalid",
			input:           "ftp://nas.example.com",
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			_, err := picoshare.NewOIDCIssuerURL(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
		})
	}
}

func TestNewOIDCRedirectURL(t *testing.T) {
	for _, tt := range []struct {
		explanation     string
		input           string
		isValidExpected bool
	}{
		{
			explanation:     "a URL ending in the callback path is valid",
			input:           "https://picoshare.example.com/oidc/callback",
			isValidExpected: true,
		},
		{
			explanation:     "a URL missing the callback path is invalid",
			input:           "https://picoshare.example.com/",
			isValidExpected: false,
		},
		{
			explanation:     "a relative URL is invalid",
			input:           "/oidc/callback",
			isValidExpected: false,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			_, err := picoshare.NewOIDCRedirectURL(tt.input)
			isValid := err == nil
			if got, want := isValid, tt.isValidExpected; got != want {
				t.Fatalf("isValid=%v, want %v (err=%v)", got, want, err)
			}
		})
	}
}

func TestOIDCRedirectURLIsHTTPS(t *testing.T) {
	httpsURL, err := picoshare.NewOIDCRedirectURL("https://picoshare.example.com/oidc/callback")
	if err != nil {
		t.Fatalf("failed to create redirect URL: %v", err)
	}
	if got, want := httpsURL.IsHTTPS(), true; got != want {
		t.Errorf("IsHTTPS()=%v, want %v", got, want)
	}

	httpURL, err := picoshare.NewOIDCRedirectURL("http://picoshare.example.com/oidc/callback")
	if err != nil {
		t.Fatalf("failed to create redirect URL: %v", err)
	}
	if got, want := httpURL.IsHTTPS(), false; got != want {
		t.Errorf("IsHTTPS()=%v, want %v", got, want)
	}
}

func TestOIDCSettingsIsConfigured(t *testing.T) {
	if got, want := (picoshare.OIDCSettings{}).IsConfigured(), false; got != want {
		t.Errorf("IsConfigured()=%v, want %v", got, want)
	}

	issuer, err := picoshare.NewOIDCIssuerURL("https://nas.example.com/webman/sso")
	if err != nil {
		t.Fatalf("failed to create issuer URL: %v", err)
	}
	if got, want := (picoshare.OIDCSettings{IssuerURL: issuer}).IsConfigured(), true; got != want {
		t.Errorf("IsConfigured()=%v, want %v", got, want)
	}
}
