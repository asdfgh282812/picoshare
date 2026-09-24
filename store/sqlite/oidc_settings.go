package sqlite

import (
	"crypto/subtle"
	"database/sql"
	"log"

	"github.com/mtlynch/picoshare/picoshare"
)

// We only store one set of OIDC settings at a time, so we use a fixed row ID,
// like the settings table.
const oidcSettingsRowID = 1

func (s Store) ReadOIDCSettings() (picoshare.OIDCSettings, error) {
	var issuerURLRaw, expectedIssuer, clientIDRaw, clientSecretRaw, redirectURLRaw, caCertPEM sql.NullString
	if err := s.db.QueryRow(`
	SELECT
		issuer_url, expected_issuer, client_id, client_secret, redirect_url, ca_cert_pem
	FROM
		oidc_settings
	WHERE
		id = :row_id`, sql.Named("row_id", oidcSettingsRowID)).Scan(
		&issuerURLRaw, &expectedIssuer, &clientIDRaw, &clientSecretRaw, &redirectURLRaw, &caCertPEM); err != nil {
		if err == sql.ErrNoRows {
			return picoshare.OIDCSettings{}, nil
		}
		return picoshare.OIDCSettings{}, err
	}

	if !issuerURLRaw.Valid {
		return picoshare.OIDCSettings{}, nil
	}

	issuerURL, err := picoshare.NewOIDCIssuerURL(issuerURLRaw.String)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}
	clientID, err := picoshare.NewOIDCClientID(clientIDRaw.String)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}
	clientSecret, err := picoshare.NewOIDCClientSecret(clientSecretRaw.String)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}
	redirectURL, err := picoshare.NewOIDCRedirectURL(redirectURLRaw.String)
	if err != nil {
		return picoshare.OIDCSettings{}, err
	}

	return picoshare.OIDCSettings{
		IssuerURL:      issuerURL,
		ExpectedIssuer: expectedIssuer.String,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		RedirectURL:    redirectURL,
		CACertPEM:      caCertPEM.String,
	}, nil
}

// UpdateOIDCSettings saves settings and marks setup as complete, so /setup
// stops accepting the one-time setup token.
func (s Store) UpdateOIDCSettings(settings picoshare.OIDCSettings) error {
	log.Printf("saving new OIDC settings (issuer=%s)", settings.IssuerURL)

	var expectedIssuer, caCertPEM sql.NullString
	if settings.ExpectedIssuer != "" {
		expectedIssuer = sql.NullString{String: settings.ExpectedIssuer, Valid: true}
	}
	if settings.CACertPEM != "" {
		caCertPEM = sql.NullString{String: settings.CACertPEM, Valid: true}
	}

	if _, err := s.db.Exec(`
	UPDATE oidc_settings
	SET
		issuer_url = :issuer_url,
		expected_issuer = :expected_issuer,
		client_id = :client_id,
		client_secret = :client_secret,
		redirect_url = :redirect_url,
		ca_cert_pem = :ca_cert_pem,
		setup_token_hash = NULL
	WHERE
		id = :row_id`,
		sql.Named("issuer_url", settings.IssuerURL.String()),
		sql.Named("expected_issuer", expectedIssuer),
		sql.Named("client_id", settings.ClientID.String()),
		sql.Named("client_secret", settings.ClientSecret.String()),
		sql.Named("redirect_url", settings.RedirectURL.String()),
		sql.Named("ca_cert_pem", caCertPEM),
		sql.Named("row_id", oidcSettingsRowID)); err != nil {
		return err
	}

	return nil
}

// ClearOIDCSettings removes PicoShare's identity provider configuration.
// Administrators trigger this by running the server with -reset-oidc when
// they're locked out because of a misconfiguration.
func (s Store) ClearOIDCSettings() error {
	log.Printf("clearing OIDC settings")
	if _, err := s.db.Exec(`
	UPDATE oidc_settings
	SET
		issuer_url = NULL,
		expected_issuer = NULL,
		client_id = NULL,
		client_secret = NULL,
		redirect_url = NULL,
		ca_cert_pem = NULL
	WHERE
		id = :row_id`, sql.Named("row_id", oidcSettingsRowID)); err != nil {
		return err
	}
	return nil
}

// NeedsSetup reports whether an administrator still needs to configure an
// identity provider.
func (s Store) NeedsSetup() (bool, error) {
	var issuerURL sql.NullString
	if err := s.db.QueryRow(`
	SELECT
		issuer_url
	FROM
		oidc_settings
	WHERE
		id = :row_id`, sql.Named("row_id", oidcSettingsRowID)).Scan(&issuerURL); err != nil {
		return false, err
	}
	return !issuerURL.Valid, nil
}

// GenerateSetupToken creates a new one-time setup token, stores its hash, and
// returns the plaintext token so the caller can print it to the server log.
func (s Store) GenerateSetupToken() (picoshare.SetupToken, error) {
	token := picoshare.NewSetupToken()
	if _, err := s.db.Exec(`
	UPDATE oidc_settings
	SET
		setup_token_hash = :hash
	WHERE
		id = :row_id`,
		sql.Named("hash", token.Hash()),
		sql.Named("row_id", oidcSettingsRowID)); err != nil {
		return picoshare.SetupToken{}, err
	}
	return token, nil
}

// ValidateSetupToken reports whether token matches the outstanding setup
// token, if any.
func (s Store) ValidateSetupToken(token picoshare.SetupToken) (bool, error) {
	var hash sql.NullString
	if err := s.db.QueryRow(`
	SELECT
		setup_token_hash
	FROM
		oidc_settings
	WHERE
		id = :row_id`, sql.Named("row_id", oidcSettingsRowID)).Scan(&hash); err != nil {
		return false, err
	}
	if !hash.Valid {
		return false, nil
	}
	return subtle.ConstantTimeCompare([]byte(hash.String), []byte(token.Hash())) == 1, nil
}
