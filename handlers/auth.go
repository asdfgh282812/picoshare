package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

var contextKeyUser = new(contextKey{name: "user"})

// sessionLifetime matches the lifetime of PicoShare's previous shared-secret
// cookie.
const sessionLifetime = 30 * 24 * time.Hour

const sessionCookieName = "session"

// oidcLoginGet starts a login by redirecting to the identity provider.
func (s Server) oidcLoginGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.identityProvider.Redirect(w, r)
	}
}

// oidcCallbackGet handles the identity provider's redirect back to PicoShare
// after a login attempt.
func (s Server) oidcCallbackGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/auth.html")

	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := s.identityProvider.Callback(w, r)
		if err != nil {
			log.Printf("OIDC login failed: %v", err)
			props, propsErr := s.authPageProps(r.Context(), "auth.loginFailed")
			if propsErr != nil {
				log.Printf("failed to check setup status: %v", propsErr)
				http.Error(w, "Failed to complete login", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusUnauthorized)
			renderTemplate(w, t, props)
			return
		}

		if err := s.completeLogin(w, r, identity); err != nil {
			log.Printf("failed to complete login for subject %s: %v", identity.Subject, err)
			http.Error(w, "Failed to complete login", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// completeLogin records identity as a PicoShare user (creating the account on
// a first login), establishes a new server-side session, and sets the
// session cookie. The dev-only login endpoint shares this with the real OIDC
// callback so both paths go through the same account-provisioning logic.
func (s Server) completeLogin(w http.ResponseWriter, r *http.Request, identity picoshare.UserIdentity) error {
	user, err := s.store.RecordUserLogin(identity)
	if err != nil {
		return fmt.Errorf("failed to record user login: %w", err)
	}

	// Revoke any session already on the request to prevent session fixation.
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if token, err := picoshare.SessionTokenFromString(cookie.Value); err == nil {
			if err := s.store.DeleteSession(token.Hash()); err != nil {
				log.Printf("failed to revoke previous session: %v", err)
			}
		}
	}

	token := picoshare.NewSessionToken()
	now := s.now()
	if err := s.store.InsertSession(picoshare.Session{
		TokenHash: token.Hash(),
		UserID:    user.ID,
		Created:   now,
		Expires:   now.Add(sessionLifetime),
	}); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	setSessionCookie(w, r, token, now.Add(sessionLifetime))

	return nil
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token picoshare.SessionToken, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token.String(),
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		// The browser arrives at the OIDC callback from a cross-site navigation
		// (the identity provider), and PicoShare's own redirect to "/" continues
		// that navigation. Browsers treat that whole chain as cross-site, so a
		// SameSite=Strict cookie wouldn't be sent on landing, and the user would
		// look logged out immediately after logging in.
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s Server) authDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			if token, err := picoshare.SessionTokenFromString(cookie.Value); err == nil {
				if err := s.store.DeleteSession(token.Hash()); err != nil {
					log.Printf("failed to delete session: %v", err)
				}
			}
		}
		clearSessionCookie(w, r)
	}
}

// loadSession reads the session cookie, if any, and attaches the
// corresponding user to the request context. It always joins against the
// users table rather than trusting anything cached in the cookie, so a
// change to a user's admin status takes effect on their very next request.
func (s Server) loadSession(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		cookie, err := r.Cookie(sessionCookieName)
		if err == nil {
			if token, err := picoshare.SessionTokenFromString(cookie.Value); err == nil {
				user, err := s.store.GetSessionUser(token.Hash())
				if err == nil {
					ctx = context.WithValue(ctx, contextKeyUser, user)
				} else if _, ok := errors.AsType[store.SessionNotFoundError](err); !ok {
					log.Printf("failed to look up session: %v", err)
				}
			}
		}

		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s Server) requireAuthentication(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuthenticated(r.Context()) {
			clearSessionCookie(w, r)
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// requireAdmin assumes requireAuthentication has already run.
func (s Server) requireAdmin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := currentUser(r.Context())
		if !user.IsAdmin {
			http.Error(w, "This page is only available to administrators", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func currentUser(ctx context.Context) (picoshare.User, bool) {
	user, ok := ctx.Value(contextKeyUser).(picoshare.User)
	return user, ok
}

func isAuthenticated(ctx context.Context) bool {
	_, ok := currentUser(ctx)
	return ok
}
