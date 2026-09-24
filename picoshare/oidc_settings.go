package picoshare

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/mtlynch/picoshare/random"
)

// setupTokenBytes matches sessionTokenBytes: 256 bits of entropy is enough
// that a stored SHA-256 hash doesn't need a slow KDF to resist brute-forcing.
const setupTokenBytes = 32

const oidcCallbackPath = "/oidc/callback"

var (
	ErrInvalidOIDCIssuerURL    = fmt.Errorf("issuer URL must be an absolute http or https URL")
	ErrInvalidOIDCRedirectURL  = fmt.Errorf("redirect URL must be an absolute http or https URL ending in %s", oidcCallbackPath)
	ErrInvalidOIDCClientID     = fmt.Errorf("client ID must not be empty")
	ErrInvalidOIDCClientSecret = fmt.Errorf("client secret must not be empty")
)

type (
	// OIDCIssuerURL is the base URL of the identity provider PicoShare uses for
	// OpenID Connect discovery.
	OIDCIssuerURL struct {
		value string
	}

	// OIDCRedirectURL is the URL the identity provider redirects back to after
	// a user authenticates.
	OIDCRedirectURL struct {
		value string
	}

	OIDCClientID struct {
		value string
	}

	OIDCClientSecret struct {
		value string
	}

	// OIDCSettings configures PicoShare's connection to a Synology SSO Server,
	// or any other standards-compliant OpenID Connect identity provider.
	// PicoShare administrators configure these settings through the web UI
	// rather than environment variables, since the settings live in the same
	// database as everything else PicoShare protects.
	OIDCSettings struct {
		IssuerURL OIDCIssuerURL
		// ExpectedIssuer overrides the issuer PicoShare expects in tokens and
		// the discovery document, for deployments where a reverse proxy makes
		// IssuerURL unreachable under its own name. Empty means "use IssuerURL."
		ExpectedIssuer string
		ClientID       OIDCClientID
		ClientSecret   OIDCClientSecret
		RedirectURL    OIDCRedirectURL
		// CACertPEM is an optional PEM-encoded certificate authority certificate,
		// for identity providers presenting certificates a public root doesn't
		// cover.
		CACertPEM string
	}
)

// IsConfigured reports whether administrators have configured PicoShare's
// connection to an identity provider yet.
func (s OIDCSettings) IsConfigured() bool {
	return s.IssuerURL.value != ""
}

func newOIDCURL(raw string, errInvalid error) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errInvalid
	}
	return raw, nil
}

func NewOIDCIssuerURL(raw string) (OIDCIssuerURL, error) {
	v, err := newOIDCURL(raw, ErrInvalidOIDCIssuerURL)
	if err != nil {
		return OIDCIssuerURL{}, err
	}
	return OIDCIssuerURL{value: v}, nil
}

func (u OIDCIssuerURL) String() string {
	return u.value
}

func NewOIDCRedirectURL(raw string) (OIDCRedirectURL, error) {
	v, err := newOIDCURL(raw, ErrInvalidOIDCRedirectURL)
	if err != nil {
		return OIDCRedirectURL{}, err
	}
	if !strings.HasSuffix(v, oidcCallbackPath) {
		return OIDCRedirectURL{}, ErrInvalidOIDCRedirectURL
	}
	return OIDCRedirectURL{value: v}, nil
}

func (u OIDCRedirectURL) String() string {
	return u.value
}

// IsHTTPS reports whether the identity provider redirects back over HTTPS,
// which determines whether PicoShare marks its cookies Secure.
func (u OIDCRedirectURL) IsHTTPS() bool {
	return strings.HasPrefix(u.value, "https://")
}

func NewOIDCClientID(raw string) (OIDCClientID, error) {
	if raw == "" {
		return OIDCClientID{}, ErrInvalidOIDCClientID
	}
	return OIDCClientID{value: raw}, nil
}

func (id OIDCClientID) String() string {
	return id.value
}

func NewOIDCClientSecret(raw string) (OIDCClientSecret, error) {
	if raw == "" {
		return OIDCClientSecret{}, ErrInvalidOIDCClientSecret
	}
	return OIDCClientSecret{value: raw}, nil
}

func (s OIDCClientSecret) String() string {
	return s.value
}

// SetupToken is the one-time secret PicoShare prints to its logs at startup
// when no administrator has configured an identity provider yet. Presenting
// it at /setup proves that whoever is configuring OIDC settings has access
// to PicoShare's server logs, not just its web interface.
type SetupToken struct {
	value string
}

// NewSetupToken generates a new, random setup token.
func NewSetupToken() SetupToken {
	return SetupToken{
		value: base64.RawURLEncoding.EncodeToString(random.Bytes(setupTokenBytes)),
	}
}

// SetupTokenFromString constructs a setup token from a value a client
// presents, such as a setup form.
func SetupTokenFromString(raw string) (SetupToken, error) {
	if raw == "" {
		return SetupToken{}, fmt.Errorf("setup token must not be empty")
	}
	return SetupToken{value: raw}, nil
}

func (t SetupToken) String() string {
	return t.value
}

// Hash returns the value PicoShare stores in the database to identify this
// token without storing the token itself.
func (t SetupToken) Hash() string {
	sum := sha256.Sum256([]byte(t.value))
	return hex.EncodeToString(sum[:])
}
