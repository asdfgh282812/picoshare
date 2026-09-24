package oidcauth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
)

// attemptCookie carries the values PicoShare needs to validate the identity
// provider's response, between Redirect and Callback. It's encoded as a
// single cookie value rather than server-side session state, since PicoShare
// hasn't identified the user yet at this point in the flow.
type attemptCookie struct {
	State    string
	Nonce    string
	Verifier string
}

func setAttemptCookie(w http.ResponseWriter, secure bool, a attemptCookie) {
	value := strings.Join([]string{
		base64.RawURLEncoding.EncodeToString([]byte(a.State)),
		base64.RawURLEncoding.EncodeToString([]byte(a.Nonce)),
		base64.RawURLEncoding.EncodeToString([]byte(a.Verifier)),
	}, ".")

	http.SetCookie(w, &http.Cookie{
		Name:     attemptCookieName,
		Value:    value,
		Path:     attemptCookiePath,
		MaxAge:   int(attemptCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAttemptCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     attemptCookieName,
		Value:    "",
		Path:     attemptCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func readAttemptCookie(r *http.Request) (attemptCookie, error) {
	cookie, err := r.Cookie(attemptCookieName)
	if err != nil {
		return attemptCookie{}, fmt.Errorf("no login attempt in progress: %w", err)
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 3 {
		return attemptCookie{}, fmt.Errorf("malformed login attempt cookie")
	}

	decoded := make([]string, 3)
	for i, p := range parts {
		raw, err := base64.RawURLEncoding.DecodeString(p)
		if err != nil {
			return attemptCookie{}, fmt.Errorf("malformed login attempt cookie: %w", err)
		}
		decoded[i] = string(raw)
	}

	return attemptCookie{
		State:    decoded[0],
		Nonce:    decoded[1],
		Verifier: decoded[2],
	}, nil
}
