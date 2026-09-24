package oidcauth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/mtlynch/picoshare/handlers/auth/oidcauth"
	"github.com/mtlynch/picoshare/picoshare"
)

// idTokenClaims mirrors the claims PicoShare reads out of an ID token, plus
// the standard claims go-oidc validates.
type idTokenClaims struct {
	Issuer   string `json:"iss"`
	Subject  string `json:"sub"`
	Audience string `json:"aud"`
	Expiry   int64  `json:"exp"`
	IssuedAt int64  `json:"iat"`
	Nonce    string `json:"nonce,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
}

// fakeIdentityProvider is a minimal OpenID Connect identity provider for
// tests: it serves discovery, a JWKS, and a token endpoint, and lets each
// test control what the token endpoint returns.
type fakeIdentityProvider struct {
	server     *httptest.Server
	signingKey *rsa.PrivateKey
	keyID      string
	// tokenResponse, when set, is served verbatim (as JSON) from the token
	// endpoint, letting a test simulate an error response.
	tokenResponse map[string]any
	tokenStatus   int
	// issuerOverride, when set, is what the discovery document claims as its
	// issuer, which may differ from the server's own URL to test issuer
	// mismatch handling.
	issuerOverride string
}

func newFakeIdentityProvider(t *testing.T) *fakeIdentityProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	f := &fakeIdentityProvider{
		signingKey: key,
		keyID:      "test-key",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", f.discoveryHandler)
	mux.HandleFunc("/jwks", f.jwksHandler)
	mux.HandleFunc("/token", f.tokenHandler)
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)

	return f
}

func (f *fakeIdentityProvider) issuer() string {
	if f.issuerOverride != "" {
		return f.issuerOverride
	}
	return f.server.URL
}

func (f *fakeIdentityProvider) discoveryHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"issuer":                                f.issuer(),
		"authorization_endpoint":                f.server.URL + "/authorize",
		"token_endpoint":                        f.server.URL + "/token",
		"jwks_uri":                              f.server.URL + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (f *fakeIdentityProvider) jwksHandler(w http.ResponseWriter, r *http.Request) {
	set := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				Key:       &f.signingKey.PublicKey,
				KeyID:     f.keyID,
				Algorithm: "RS256",
				Use:       "sig",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(set)
}

func (f *fakeIdentityProvider) tokenHandler(w http.ResponseWriter, r *http.Request) {
	if f.tokenStatus != 0 {
		w.WriteHeader(f.tokenStatus)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Enforce PKCE: the verifier the client sends must hash to the challenge
	// it advertised when starting the login.
	verifier := r.PostForm.Get("code_verifier")
	wantChallenge := r.PostForm.Get("_want_challenge")
	if wantChallenge != "" && s256(verifier) != wantChallenge {
		http.Error(w, "PKCE verification failed", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(f.tokenResponse)
}

func (f *fakeIdentityProvider) sign(t *testing.T, claims idTokenClaims) string {
	t.Helper()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: f.signingKey},
		(&jose.SignerOptions{}).WithHeader("kid", f.keyID),
	)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("failed to sign ID token: %v", err)
	}
	return raw
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type fakeSettingsReader struct {
	settings picoshare.OIDCSettings
}

func (r fakeSettingsReader) ReadOIDCSettings() (picoshare.OIDCSettings, error) {
	return r.settings, nil
}

func mustSettings(t *testing.T, issuer, expectedIssuer, redirectURL string) picoshare.OIDCSettings {
	t.Helper()
	issuerURL, err := picoshare.NewOIDCIssuerURL(issuer)
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
	redirect, err := picoshare.NewOIDCRedirectURL(redirectURL)
	if err != nil {
		t.Fatalf("failed to create redirect URL: %v", err)
	}
	return picoshare.OIDCSettings{
		IssuerURL:      issuerURL,
		ExpectedIssuer: expectedIssuer,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURL:    redirect,
	}
}

// startLogin drives Client.Redirect and returns the state, nonce, and PKCE
// challenge it generated, plus the attempt cookie the browser would carry to
// the callback.
func startLogin(t *testing.T, c *oidcauth.Client) (state, nonce, challenge string, attemptCookie *http.Cookie) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/oidc/login", nil)
	rec := httptest.NewRecorder()
	c.Redirect(rec, req)

	res := rec.Result()
	if got, want := res.StatusCode, http.StatusFound; got != want {
		t.Fatalf("Redirect status=%d, want=%d", got, want)
	}

	loc, err := url.Parse(res.Header.Get("Location"))
	if err != nil {
		t.Fatalf("failed to parse redirect Location: %v", err)
	}
	q := loc.Query()

	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies from Redirect, want 1", len(cookies))
	}

	return q.Get("state"), q.Get("nonce"), q.Get("code_challenge"), cookies[0]
}

func TestCallbackSucceedsWithValidToken(t *testing.T) {
	idp := newFakeIdentityProvider(t)
	settings := mustSettings(t, idp.server.URL, "", "https://picoshare.example.com/oidc/callback")
	now := time.Date(2025, time.June, 1, 12, 0, 0, 0, time.UTC)
	c := oidcauth.New(fakeSettingsReader{settings}, func() time.Time { return now })

	state, nonce, challenge, attemptCookie := startLogin(t, c)

	idToken := idp.sign(t, idTokenClaims{
		Issuer:   idp.issuer(),
		Subject:  "synology-user-1",
		Audience: "test-client",
		Expiry:   now.Add(time.Hour).Unix(),
		IssuedAt: now.Unix(),
		Nonce:    nonce,
		Username: "alice",
		Email:    "alice@example.com",
	})
	idp.tokenResponse = map[string]any{
		"access_token":    "dummy-access-token",
		"token_type":      "Bearer",
		"id_token":        idToken,
		"_want_challenge": challenge,
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/oidc/callback?code=dummy-code&state=%s", state), nil)
	req.AddCookie(attemptCookie)
	rec := httptest.NewRecorder()

	identity, err := c.Callback(rec, req)
	if err != nil {
		t.Fatalf("Callback failed: %v", err)
	}

	if got, want := identity.Subject.String(), "synology-user-1"; got != want {
		t.Errorf("Subject=%s, want=%s", got, want)
	}
	if got, want := identity.Username.String(), "alice"; got != want {
		t.Errorf("Username=%s, want=%s", got, want)
	}
	if got, want := identity.Email.String(), "alice@example.com"; got != want {
		t.Errorf("Email=%s, want=%s", got, want)
	}
}

func TestCallbackRejectsInvalidLogins(t *testing.T) {
	baseNow := time.Date(2025, time.June, 1, 12, 0, 0, 0, time.UTC)

	newValidClaims := func(idp *fakeIdentityProvider, nonce string) idTokenClaims {
		return idTokenClaims{
			Issuer:   idp.issuer(),
			Subject:  "synology-user-1",
			Audience: "test-client",
			Expiry:   baseNow.Add(time.Hour).Unix(),
			IssuedAt: baseNow.Unix(),
			Nonce:    nonce,
			Username: "alice",
			Email:    "alice@example.com",
		}
	}

	for _, tt := range []struct {
		explanation string
		// mutate lets a test case tamper with the otherwise-valid setup before
		// the callback request is sent.
		mutate func(idp *fakeIdentityProvider, claims *idTokenClaims, callbackURL *url.URL, cookie *http.Cookie)
		// signWithWrongKey signs the ID token with a key the identity provider
		// never advertises in its JWKS.
		signWithWrongKey bool
		dropCookie       bool
		tokenErrorStatus int
	}{
		{
			explanation: "state parameter doesn't match the login attempt",
			mutate: func(_ *fakeIdentityProvider, _ *idTokenClaims, callbackURL *url.URL, _ *http.Cookie) {
				q := callbackURL.Query()
				q.Set("state", "attacker-supplied-state")
				callbackURL.RawQuery = q.Encode()
			},
		},
		{
			explanation: "nonce in the ID token doesn't match the login attempt",
			mutate: func(_ *fakeIdentityProvider, claims *idTokenClaims, _ *url.URL, _ *http.Cookie) {
				claims.Nonce = "attacker-supplied-nonce"
			},
		},
		{
			explanation: "audience doesn't match the client ID",
			mutate: func(_ *fakeIdentityProvider, claims *idTokenClaims, _ *url.URL, _ *http.Cookie) {
				claims.Audience = "someone-elses-client"
			},
		},
		{
			explanation: "token has already expired",
			mutate: func(_ *fakeIdentityProvider, claims *idTokenClaims, _ *url.URL, _ *http.Cookie) {
				claims.Expiry = baseNow.Add(-time.Hour).Unix()
				claims.IssuedAt = baseNow.Add(-2 * time.Hour).Unix()
			},
		},
		{
			explanation: "missing username claim",
			mutate: func(_ *fakeIdentityProvider, claims *idTokenClaims, _ *url.URL, _ *http.Cookie) {
				claims.Username = ""
			},
		},
		{
			explanation:      "signed with a key the identity provider never published",
			signWithWrongKey: true,
		},
		{
			explanation: "login attempt cookie is missing",
			dropCookie:  true,
		},
		{
			explanation:      "identity provider rejects the authorization code",
			tokenErrorStatus: http.StatusBadRequest,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			idp := newFakeIdentityProvider(t)
			settings := mustSettings(t, idp.server.URL, "", "https://picoshare.example.com/oidc/callback")
			c := oidcauth.New(fakeSettingsReader{settings}, func() time.Time { return baseNow })

			state, nonce, challenge, attemptCookie := startLogin(t, c)

			claims := newValidClaims(idp, nonce)

			callbackURL, err := url.Parse(fmt.Sprintf("/oidc/callback?code=dummy-code&state=%s", state))
			if err != nil {
				t.Fatalf("failed to parse callback URL: %v", err)
			}

			if tt.mutate != nil {
				tt.mutate(idp, &claims, callbackURL, attemptCookie)
			}

			if tt.tokenErrorStatus != 0 {
				idp.tokenStatus = tt.tokenErrorStatus
			} else {
				signingKey := idp.signingKey
				keyID := idp.keyID
				if tt.signWithWrongKey {
					wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
					if err != nil {
						t.Fatalf("failed to generate substitute key: %v", err)
					}
					idp.signingKey = wrongKey
				}
				idToken := idp.sign(t, claims)
				idp.signingKey, idp.keyID = signingKey, keyID

				idp.tokenResponse = map[string]any{
					"access_token":    "dummy-access-token",
					"token_type":      "Bearer",
					"id_token":        idToken,
					"_want_challenge": challenge,
				}
			}

			req := httptest.NewRequest(http.MethodGet, callbackURL.String(), nil)
			if !tt.dropCookie {
				req.AddCookie(attemptCookie)
			}
			rec := httptest.NewRecorder()

			if _, err := c.Callback(rec, req); err == nil {
				t.Fatalf("Callback succeeded, want an error")
			}
		})
	}
}

func TestCallbackWithIdentityProviderError(t *testing.T) {
	idp := newFakeIdentityProvider(t)
	settings := mustSettings(t, idp.server.URL, "", "https://picoshare.example.com/oidc/callback")
	now := time.Date(2025, time.June, 1, 12, 0, 0, 0, time.UTC)
	c := oidcauth.New(fakeSettingsReader{settings}, func() time.Time { return now })

	_, _, _, attemptCookie := startLogin(t, c)

	req := httptest.NewRequest(http.MethodGet, "/oidc/callback?error=access_denied&error_description=user+declined", nil)
	req.AddCookie(attemptCookie)
	rec := httptest.NewRecorder()

	if _, err := c.Callback(rec, req); err == nil {
		t.Fatal("Callback succeeded, want an error for an identity-provider-reported failure")
	}
}

func TestCallbackWithoutStartingLogin(t *testing.T) {
	idp := newFakeIdentityProvider(t)
	settings := mustSettings(t, idp.server.URL, "", "https://picoshare.example.com/oidc/callback")
	c := oidcauth.New(fakeSettingsReader{settings}, time.Now)

	req := httptest.NewRequest(http.MethodGet, "/oidc/callback?code=dummy-code&state=dummy-state", nil)
	rec := httptest.NewRecorder()

	if _, err := c.Callback(rec, req); err == nil {
		t.Fatal("Callback succeeded without an attempt cookie, want an error")
	}
}

func TestIssuerMismatch(t *testing.T) {
	idp := newFakeIdentityProvider(t)
	idp.issuerOverride = "https://not-the-real-issuer.example.com"

	t.Run("rejected without an override", func(t *testing.T) {
		settings := mustSettings(t, idp.server.URL, "", "https://picoshare.example.com/oidc/callback")
		c := oidcauth.New(fakeSettingsReader{settings}, time.Now)

		req := httptest.NewRequest(http.MethodGet, "/oidc/login", nil)
		rec := httptest.NewRecorder()
		c.Redirect(rec, req)

		// An issuer mismatch fails discovery, so Redirect falls back to sending
		// the browser back to the login page instead of the (nonexistent)
		// identity provider.
		if got, want := rec.Result().Header.Get("Location"), "/login?error=not_configured"; got != want {
			t.Errorf("Location=%q, want=%q", got, want)
		}
	})

	t.Run("accepted with a matching override", func(t *testing.T) {
		settings := mustSettings(t, idp.server.URL, idp.issuerOverride, "https://picoshare.example.com/oidc/callback")
		c := oidcauth.New(fakeSettingsReader{settings}, time.Now)

		req := httptest.NewRequest(http.MethodGet, "/oidc/login", nil)
		rec := httptest.NewRecorder()
		c.Redirect(rec, req)

		if got, want := rec.Result().StatusCode, http.StatusFound; got != want {
			t.Fatalf("status=%d, want=%d", got, want)
		}
	})
}

func TestDiscover(t *testing.T) {
	idp := newFakeIdentityProvider(t)

	t.Run("succeeds against a valid issuer", func(t *testing.T) {
		settings := mustSettings(t, idp.server.URL, "", "https://picoshare.example.com/oidc/callback")
		if err := oidcauth.Discover(t.Context(), settings); err != nil {
			t.Errorf("Discover failed: %v", err)
		}
	})

	t.Run("fails against an unreachable issuer", func(t *testing.T) {
		settings := mustSettings(t, "https://127.0.0.1:1/webman/sso", "", "https://picoshare.example.com/oidc/callback")
		if err := oidcauth.Discover(t.Context(), settings); err == nil {
			t.Error("Discover succeeded against an unreachable issuer, want an error")
		}
	})
}
