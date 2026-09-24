// Package oidcauth drives PicoShare's OpenID Connect login flow: it's
// written and tested against Synology SSO Server, but works with any
// standards-compliant identity provider.
//
// Unlike most OIDC clients, this one doesn't take its configuration at
// startup. PicoShare administrators configure their identity provider
// through the web UI, so the settings live in the same database as
// everything else PicoShare protects, and Client re-reads them on every
// login rather than requiring a restart when they change.
package oidcauth

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/random"
)

const (
	// attemptCookieName holds the state, nonce, and PKCE verifier for a login
	// attempt in progress. It's short-lived and scoped to the OIDC endpoints
	// only.
	attemptCookieName = "ps_oidc"
	attemptCookiePath = "/oidc/"
	attemptCookieTTL  = 10 * time.Minute

	requestTimeout = 15 * time.Second
)

var (
	// ErrNotConfigured means an administrator hasn't finished setting up an
	// identity provider yet.
	ErrNotConfigured = fmt.Errorf("OIDC is not configured")
	// ErrLoginFailed wraps every way a login attempt can go wrong once the
	// browser is back at the callback endpoint, so handlers can show the user a
	// generic message while the server log keeps the detail.
	ErrLoginFailed = fmt.Errorf("login failed")
)

// SettingsReader supplies PicoShare's current identity provider
// configuration. store.Store satisfies this.
type SettingsReader interface {
	ReadOIDCSettings() (picoshare.OIDCSettings, error)
}

// Client implements handlers.IdentityProvider.
type Client struct {
	settings SettingsReader
	now      func() time.Time

	mu       sync.Mutex
	provider *cachedProvider
}

// cachedProvider is the result of OIDC discovery against a particular
// OIDCSettings value. Client discards it and re-discovers whenever the
// settings change.
type cachedProvider struct {
	fingerprint string
	settings    picoshare.OIDCSettings
	httpClient  *http.Client
	provider    *oidc.Provider
	verifier    *oidc.IDTokenVerifier
	oauthConfig oauth2.Config
}

func New(settings SettingsReader, now func() time.Time) *Client {
	return &Client{
		settings: settings,
		now:      now,
	}
}

// Redirect sends the browser to the identity provider to begin a login. If
// OIDC isn't configured yet, it sends the browser back to the login page with
// an explanation instead.
func (c *Client) Redirect(w http.ResponseWriter, r *http.Request) {
	cp, err := c.ensureProvider(r.Context())
	if err != nil {
		log.Printf("cannot start OIDC login: %v", err)
		http.Redirect(w, r, "/login?error=not_configured", http.StatusFound)
		return
	}

	state := randomURLSafeString()
	nonce := randomURLSafeString()
	verifier := oauth2.GenerateVerifier()

	setAttemptCookie(w, cp.settings.RedirectURL.IsHTTPS(), attemptCookie{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
	})

	authCodeURL := cp.oauthConfig.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.S256ChallengeOption(verifier))
	http.Redirect(w, r, authCodeURL, http.StatusFound)
}

// Callback validates the identity provider's response to a login attempt
// started by Redirect and returns the identity it asserts for the user.
func (c *Client) Callback(w http.ResponseWriter, r *http.Request) (picoshare.UserIdentity, error) {
	// Always clear the attempt cookie: it's single-use whether or not the
	// login succeeds.
	attempt, cookieErr := readAttemptCookie(r)
	clearAttemptCookie(w)

	cp, err := c.ensureProvider(r.Context())
	if err != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: %w", ErrLoginFailed, err)
	}

	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: identity provider returned error %q: %s", ErrLoginFailed, errMsg, r.URL.Query().Get("error_description"))
	}

	if cookieErr != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: missing or invalid login attempt cookie: %w", ErrLoginFailed, cookieErr)
	}

	state := r.URL.Query().Get("state")
	if subtle.ConstantTimeCompare([]byte(state), []byte(attempt.State)) != 1 {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: state mismatch", ErrLoginFailed)
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: missing authorization code", ErrLoginFailed)
	}

	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, cp.httpClient)
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	token, err := cp.oauthConfig.Exchange(ctx, code, oauth2.VerifierOption(attempt.Verifier))
	if err != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: failed to exchange authorization code: %w", ErrLoginFailed, err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: token response did not include an id_token", ErrLoginFailed)
	}

	idToken, err := cp.verifier.Verify(oidc.ClientContext(ctx, cp.httpClient), rawIDToken)
	if err != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: failed to verify ID token: %w", ErrLoginFailed, err)
	}

	var claims struct {
		Nonce    string `json:"nonce"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: failed to parse ID token claims: %w", ErrLoginFailed, err)
	}
	if subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(attempt.Nonce)) != 1 {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: nonce mismatch", ErrLoginFailed)
	}

	subject, err := picoshare.NewOIDCSubject(idToken.Subject)
	if err != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: invalid subject claim: %w", ErrLoginFailed, err)
	}
	if claims.Username == "" {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: identity provider did not supply a username claim", ErrLoginFailed)
	}
	username, err := picoshare.NewUsername(claims.Username)
	if err != nil {
		return picoshare.UserIdentity{}, fmt.Errorf("%w: invalid username claim: %w", ErrLoginFailed, err)
	}
	email := picoshare.NoEmailAddress
	if claims.Email != "" {
		if email, err = picoshare.NewEmailAddress(claims.Email); err != nil {
			log.Printf("identity provider supplied an unusable email claim %q for subject %s: %v", claims.Email, idToken.Subject, err)
			email = picoshare.NoEmailAddress
		}
	}

	return picoshare.UserIdentity{
		Subject:  subject,
		Username: username,
		Email:    email,
	}, nil
}

// ensureProvider returns a provider discovered from PicoShare's current OIDC
// settings, discovering (or re-discovering, if settings changed) as needed.
func (c *Client) ensureProvider(ctx context.Context) (*cachedProvider, error) {
	settings, err := c.settings.ReadOIDCSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to read OIDC settings: %w", err)
	}
	if !settings.IsConfigured() {
		return nil, ErrNotConfigured
	}

	fingerprint := settingsFingerprint(settings)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.provider != nil && c.provider.fingerprint == fingerprint {
		return c.provider, nil
	}

	httpClient, err := httpClientFor(settings)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP client for identity provider: %w", err)
	}

	discoveryCtx := oidc.ClientContext(context.Background(), httpClient)
	if settings.ExpectedIssuer != "" {
		discoveryCtx = oidc.InsecureIssuerURLContext(discoveryCtx, settings.ExpectedIssuer)
	}
	discoveryCtx, cancel := context.WithTimeout(discoveryCtx, requestTimeout)
	defer cancel()

	provider, err := oidc.NewProvider(discoveryCtx, settings.IssuerURL.String())
	if err != nil {
		return nil, fmt.Errorf("failed to discover identity provider at %s: %w", settings.IssuerURL, err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: settings.ClientID.String(),
		Now:      c.now,
	})

	cp := &cachedProvider{
		fingerprint: fingerprint,
		settings:    settings,
		httpClient:  httpClient,
		provider:    provider,
		verifier:    verifier,
		oauthConfig: oauth2.Config{
			ClientID:     settings.ClientID.String(),
			ClientSecret: settings.ClientSecret.String(),
			RedirectURL:  settings.RedirectURL.String(),
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "email"},
		},
	}
	c.provider = cp
	return cp, nil
}

// Discover attempts OpenID Connect discovery against settings and returns an
// error describing why it failed, if it did. Administrators use this from
// the setup and settings pages to check their configuration before saving
// it, without needing to complete a full login.
func Discover(ctx context.Context, settings picoshare.OIDCSettings) error {
	httpClient, err := httpClientFor(settings)
	if err != nil {
		return err
	}

	discoveryCtx := oidc.ClientContext(ctx, httpClient)
	if settings.ExpectedIssuer != "" {
		discoveryCtx = oidc.InsecureIssuerURLContext(discoveryCtx, settings.ExpectedIssuer)
	}
	discoveryCtx, cancel := context.WithTimeout(discoveryCtx, requestTimeout)
	defer cancel()

	if _, err := oidc.NewProvider(discoveryCtx, settings.IssuerURL.String()); err != nil {
		return err
	}
	return nil
}

func httpClientFor(settings picoshare.OIDCSettings) (*http.Client, error) {
	if settings.CACertPEM == "" {
		return &http.Client{Timeout: requestTimeout}, nil
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM([]byte(settings.CACertPEM)) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}, nil
}

// settingsFingerprint identifies which OIDCSettings value produced a cached
// provider, so a settings change invalidates the cache without needing a
// restart.
func settingsFingerprint(s picoshare.OIDCSettings) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		s.IssuerURL, s.ExpectedIssuer, s.ClientID, s.ClientSecret, s.RedirectURL, s.CACertPEM)
}

func randomURLSafeString() string {
	return base64.RawURLEncoding.EncodeToString(random.Bytes(32))
}
