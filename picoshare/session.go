package picoshare

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mtlynch/picoshare/random"
)

// sessionTokenBytes is the amount of entropy in a session token. 256 bits is
// enough that a stored SHA-256 hash doesn't need a slow KDF to resist
// brute-forcing.
const sessionTokenBytes = 32

var ErrInvalidSessionToken = fmt.Errorf("session token is malformed")

type (
	// SessionToken is the secret value PicoShare stores in a user's session
	// cookie. PicoShare never stores the token itself, only its hash.
	SessionToken struct {
		value string
	}

	// SessionTokenHash is the SHA-256 hash of a session token, which is what
	// PicoShare stores in the database.
	SessionTokenHash struct {
		value string
	}

	Session struct {
		TokenHash SessionTokenHash
		UserID    UserID
		Created   time.Time
		Expires   time.Time
	}
)

// NewSessionToken generates a new, random session token.
func NewSessionToken() SessionToken {
	return SessionToken{
		value: base64.RawURLEncoding.EncodeToString(random.Bytes(sessionTokenBytes)),
	}
}

// SessionTokenFromString constructs a session token from a value a client
// presents, such as a cookie.
func SessionTokenFromString(raw string) (SessionToken, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(decoded) != sessionTokenBytes {
		return SessionToken{}, ErrInvalidSessionToken
	}
	return SessionToken{value: raw}, nil
}

func (t SessionToken) String() string {
	return t.value
}

// Hash returns the value PicoShare stores in the database to identify this
// token without storing the token itself.
func (t SessionToken) Hash() SessionTokenHash {
	sum := sha256.Sum256([]byte(t.value))
	return SessionTokenHash{value: hex.EncodeToString(sum[:])}
}

// SessionTokenHashFromString constructs a session token hash from a trusted
// source, such as a database row.
func SessionTokenHashFromString(raw string) SessionTokenHash {
	return SessionTokenHash{value: raw}
}

func (h SessionTokenHash) String() string {
	return h.value
}
