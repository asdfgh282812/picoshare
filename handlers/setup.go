package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/mtlynch/picoshare/handlers/auth/oidcauth"
	"github.com/mtlynch/picoshare/picoshare"
)

type oidcSettingsRequest struct {
	SetupToken     picoshare.SetupToken
	IssuerURL      picoshare.OIDCIssuerURL
	ExpectedIssuer string
	ClientID       picoshare.OIDCClientID
	ClientSecret   picoshare.OIDCClientSecret
	RedirectURL    picoshare.OIDCRedirectURL
	CACertPEM      string
}

// setupGet serves the one-time setup form administrators use to connect
// PicoShare to their identity provider. It's a 404 once setup is complete, so
// there's no ongoing attack surface from it.
func (s Server) setupGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/setup.html")

	return func(w http.ResponseWriter, r *http.Request) {
		needsSetup, err := s.store.NeedsSetup()
		if err != nil {
			log.Printf("failed to check setup status: %v", err)
			http.Error(w, "Failed to check setup status", http.StatusInternalServerError)
			return
		}
		if !needsSetup {
			http.NotFound(w, r)
			return
		}

		renderTemplate(w, t, struct {
			commonProps
			RedirectURL string
		}{
			commonProps: makeCommonProps("title.setup", r.Context()),
			RedirectURL: baseURLFromRequest(r) + "/oidc/callback",
		})
	}
}

func (s Server) setupTestConnectionPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := parseOIDCSettingsRequest(r, s.store)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		if err := oidcauth.Discover(r.Context(), oidcSettingsFromRequest(req)); err != nil {
			http.Error(w, fmt.Sprintf("Connection test failed: %v", err), http.StatusBadRequest)
			return
		}
	}
}

func (s Server) setupPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := parseOIDCSettingsRequest(r, s.store)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		if err := s.store.UpdateOIDCSettings(oidcSettingsFromRequest(req)); err != nil {
			log.Printf("failed to save OIDC settings: %v", err)
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}
	}
}

func oidcSettingsFromRequest(req oidcSettingsRequest) picoshare.OIDCSettings {
	return picoshare.OIDCSettings{
		IssuerURL:      req.IssuerURL,
		ExpectedIssuer: req.ExpectedIssuer,
		ClientID:       req.ClientID,
		ClientSecret:   req.ClientSecret,
		RedirectURL:    req.RedirectURL,
		CACertPEM:      req.CACertPEM,
	}
}

// setupTokenValidator checks a setup token against the store. Both
// parseOIDCSettingsRequest call sites (test-connection and the final save)
// require a valid token, so that only someone with access to PicoShare's
// server logs can configure or probe its identity provider.
type setupTokenValidator interface {
	ValidateSetupToken(picoshare.SetupToken) (bool, error)
}

func parseOIDCSettingsRequest(r *http.Request, validator setupTokenValidator) (oidcSettingsRequest, error) {
	var payload struct {
		SetupToken     string `json:"setupToken"`
		IssuerURL      string `json:"issuerUrl"`
		ExpectedIssuer string `json:"expectedIssuer"`
		ClientID       string `json:"clientId"`
		ClientSecret   string `json:"clientSecret"`
		RedirectURL    string `json:"redirectUrl"`
		CACertPEM      string `json:"caCertPem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return oidcSettingsRequest{}, err
	}

	setupToken, err := picoshare.SetupTokenFromString(payload.SetupToken)
	if err != nil {
		return oidcSettingsRequest{}, err
	}
	valid, err := validator.ValidateSetupToken(setupToken)
	if err != nil {
		return oidcSettingsRequest{}, err
	}
	if !valid {
		return oidcSettingsRequest{}, fmt.Errorf("invalid or expired setup token")
	}

	issuerURL, err := picoshare.NewOIDCIssuerURL(payload.IssuerURL)
	if err != nil {
		return oidcSettingsRequest{}, err
	}
	clientID, err := picoshare.NewOIDCClientID(payload.ClientID)
	if err != nil {
		return oidcSettingsRequest{}, err
	}
	clientSecret, err := picoshare.NewOIDCClientSecret(payload.ClientSecret)
	if err != nil {
		return oidcSettingsRequest{}, err
	}
	redirectURL, err := picoshare.NewOIDCRedirectURL(payload.RedirectURL)
	if err != nil {
		return oidcSettingsRequest{}, err
	}

	return oidcSettingsRequest{
		SetupToken:     setupToken,
		IssuerURL:      issuerURL,
		ExpectedIssuer: payload.ExpectedIssuer,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURL:    redirectURL,
		CACertPEM:      payload.CACertPEM,
	}, nil
}

// oidcSettingsReader is the subset of the store an admin OIDC settings
// request needs to resolve a blank client secret.
type oidcSettingsReader interface {
	ReadOIDCSettings() (picoshare.OIDCSettings, error)
}

// oidcSettingsFromAdminRequest parses an OIDC settings update from an
// already-authenticated administrator, who proves their access through their
// session rather than a setup token. A blank client secret means "keep the
// currently configured secret": the admin settings page never echoes the
// secret back, so there's no other way for an admin to resubmit it unchanged.
func oidcSettingsFromAdminRequest(r *http.Request, current oidcSettingsReader) (picoshare.OIDCSettings, error) {
	var payload struct {
		IssuerURL      string `json:"issuerUrl"`
		ExpectedIssuer string `json:"expectedIssuer"`
		ClientID       string `json:"clientId"`
		ClientSecret   string `json:"clientSecret"`
		RedirectURL    string `json:"redirectUrl"`
		CACertPEM      string `json:"caCertPem"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return picoshare.OIDCSettings{}, err
	}

	issuerURL, err := picoshare.NewOIDCIssuerURL(payload.IssuerURL)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}
	clientID, err := picoshare.NewOIDCClientID(payload.ClientID)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}
	clientSecret, err := resolveAdminClientSecret(payload.ClientSecret, current)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}
	redirectURL, err := picoshare.NewOIDCRedirectURL(payload.RedirectURL)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}

	return picoshare.OIDCSettings{
		IssuerURL:      issuerURL,
		ExpectedIssuer: payload.ExpectedIssuer,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURL:    redirectURL,
		CACertPEM:      payload.CACertPEM,
	}, nil
}

func resolveAdminClientSecret(raw string, current oidcSettingsReader) (picoshare.OIDCClientSecret, error) {
	if raw != "" {
		return picoshare.NewOIDCClientSecret(raw)
	}
	existing, err := current.ReadOIDCSettings()
	if err != nil {
		return picoshare.OIDCClientSecret{}, err
	}
	if existing.ClientSecret.String() == "" {
		return picoshare.OIDCClientSecret{}, picoshare.ErrInvalidOIDCClientSecret
	}
	return existing.ClientSecret, nil
}

// adminOIDCSettingsGet returns the current OIDC settings so the Settings page
// can show what's configured. It never returns the client secret or CA
// certificate, since an administrator has no need to read them back.
func (s Server) adminOIDCSettingsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := s.store.ReadOIDCSettings()
		if err != nil {
			log.Printf("failed to read OIDC settings: %v", err)
			http.Error(w, "Failed to read settings", http.StatusInternalServerError)
			return
		}
		respondJSON(w, struct {
			IssuerURL      string `json:"issuerUrl"`
			ExpectedIssuer string `json:"expectedIssuer"`
			ClientID       string `json:"clientId"`
			RedirectURL    string `json:"redirectUrl"`
			HasCACert      bool   `json:"hasCaCert"`
		}{
			IssuerURL:      settings.IssuerURL.String(),
			ExpectedIssuer: settings.ExpectedIssuer,
			ClientID:       settings.ClientID.String(),
			RedirectURL:    settings.RedirectURL.String(),
			HasCACert:      settings.CACertPEM != "",
		})
	}
}

func (s Server) adminOIDCSettingsPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := oidcSettingsFromAdminRequest(r, s.store)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}
		if err := s.store.UpdateOIDCSettings(settings); err != nil {
			log.Printf("failed to save OIDC settings: %v", err)
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}
	}
}

func (s Server) adminOIDCSettingsTestConnectionPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := oidcSettingsFromAdminRequest(r, s.store)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}
		if err := oidcauth.Discover(r.Context(), settings); err != nil {
			http.Error(w, fmt.Sprintf("Connection test failed: %v", err), http.StatusBadRequest)
			return
		}
	}
}

// ssoSettingsGet serves the page administrators use to review and update
// PicoShare's identity provider connection after initial setup. Unlike
// setupGet, it never 404s once SSO is configured -- misconfigurations happen
// after the fact too, and /setup is no longer reachable to fix them.
func (s Server) ssoSettingsGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/sso-settings.html")

	return func(w http.ResponseWriter, r *http.Request) {
		renderTemplate(w, t, struct {
			commonProps
			RedirectURL string
		}{
			commonProps: makeCommonProps("title.ssoSettings", r.Context()),
			RedirectURL: baseURLFromRequest(r) + "/oidc/callback",
		})
	}
}
