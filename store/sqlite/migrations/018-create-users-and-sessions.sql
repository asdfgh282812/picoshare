-- Add multi-user support: PicoShare users authenticate through a single
-- OpenID Connect identity provider (for example, Synology SSO Server)
-- instead of a shared passphrase.

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    oidc_subject TEXT NOT NULL UNIQUE CHECK (
        length(oidc_subject) BETWEEN 1 AND 255
    ),
    username TEXT NOT NULL CHECK (length(username) BETWEEN 1 AND 200),
    -- email is NULL when the identity provider didn't supply one.
    email TEXT CHECK (email IS NULL OR length(email) BETWEEN 3 AND 320),
    is_admin INTEGER NOT NULL CHECK (is_admin IN (0, 1)) DEFAULT 0,
    creation_time TEXT NOT NULL CHECK (
        datetime(creation_time) IS NOT NULL
        AND datetime(creation_time) >= datetime('2022-02-20')
    ),
    last_login_time TEXT NOT NULL CHECK (
        datetime(last_login_time) IS NOT NULL
        AND datetime(last_login_time) >= datetime('2022-02-20')
    )
) STRICT;

-- token_hash is the hex-encoded SHA-256 hash of a session token. PicoShare
-- never stores the token itself, only its hash, so a database leak alone
-- doesn't let an attacker impersonate a session.
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY CHECK (length(token_hash) = 64),
    user_id INTEGER NOT NULL REFERENCES users (id),
    creation_time TEXT NOT NULL CHECK (
        datetime(creation_time) IS NOT NULL
        AND datetime(creation_time) >= datetime('2022-02-20')
    ),
    expiration_time TEXT NOT NULL CHECK (
        datetime(expiration_time) IS NOT NULL
        AND datetime(expiration_time) >= datetime('2022-02-20')
    )
) STRICT;

CREATE INDEX idx_sessions_user_id ON sessions (user_id);

CREATE INDEX idx_sessions_expiration_time ON sessions (expiration_time);

-- oidc_settings holds PicoShare's connection to its identity provider.
-- PicoShare only ever stores one set of OIDC settings, so it uses a fixed row
-- ID, like the settings table. All columns are NULL until an administrator
-- completes setup.
CREATE TABLE oidc_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    issuer_url TEXT,
    expected_issuer TEXT,
    client_id TEXT,
    -- client_secret is a confidential OAuth client secret, not a user
    -- credential, so PicoShare stores it in plaintext to pass it back to the
    -- identity provider during token exchange.
    client_secret TEXT,
    redirect_url TEXT,
    ca_cert_pem TEXT,
    -- setup_token_hash is the hex-encoded SHA-256 hash of the one-time setup
    -- token PicoShare prints to its logs at startup when no administrator has
    -- configured OIDC settings yet. It's NULL once setup is complete.
    setup_token_hash TEXT CHECK (
        setup_token_hash IS NULL OR length(setup_token_hash) = 64
    )
) STRICT;

INSERT INTO oidc_settings (id) VALUES (1);
