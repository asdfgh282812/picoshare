package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store/sqlite"
)

// userStore is the subset of sqlite.Store that test helpers need to log a
// user in directly, bypassing the OIDC flow entirely. handlers/auth/oidcauth
// has its own tests covering the real OIDC callback.
type userStore interface {
	RecordUserLogin(picoshare.UserIdentity) (picoshare.User, error)
	InsertSession(picoshare.Session) error
}

// mustLoginAsUser records a login for subject and returns the resulting user
// plus a session cookie a test can attach to requests with req.AddCookie.
// Logging in the first user of a test's store makes them an administrator;
// log in additional users to get non-admin accounts.
func mustLoginAsUser(t testing.TB, s userStore, subject string, now time.Time) (picoshare.User, *http.Cookie) {
	t.Helper()

	if now.IsZero() {
		// Some tests pass a zero time because they don't care about it (for
		// example, requests that fail validation before any time-based logic
		// runs). A zero time would otherwise fail the sessions table's epoch
		// check.
		now = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	}

	identity := picoshare.UserIdentity{
		Subject:  mustOIDCSubject(t, subject),
		Username: mustUsername(t, subject),
	}
	user, err := s.RecordUserLogin(identity)
	if err != nil {
		t.Fatalf("failed to record user login: %v", err)
	}

	token := picoshare.NewSessionToken()
	if err := s.InsertSession(picoshare.Session{
		TokenHash: token.Hash(),
		UserID:    user.ID,
		Created:   now,
		// Expires is independent of now (which tests often set to a date
		// convenient for unrelated assertions, such as an expiration test
		// fixture) and of test_sqlite's fixed clock (2025-01-01), so it must be
		// far enough in the future to outlast both.
		Expires: time.Date(2999, time.January, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("failed to insert session: %v", err)
	}

	return user, &http.Cookie{Name: "session", Value: token.String()}
}

// mustLoginAsAdmin is a convenience wrapper for the common case: a single
// user who becomes PicoShare's sole administrator by logging in first.
func mustLoginAsAdmin(t testing.TB, s *sqlite.Store, now time.Time) *http.Cookie {
	t.Helper()
	_, cookie := mustLoginAsUser(t, s, "admin", now)
	return cookie
}

func mustOIDCSubject(t testing.TB, raw string) picoshare.OIDCSubject {
	t.Helper()
	s, err := picoshare.NewOIDCSubject(raw)
	if err != nil {
		t.Fatalf("failed to create OIDC subject: %v", err)
	}
	return s
}

func mustUsername(t testing.TB, raw string) picoshare.Username {
	t.Helper()
	u, err := picoshare.NewUsername(raw)
	if err != nil {
		t.Fatalf("failed to create username: %v", err)
	}
	return u
}
