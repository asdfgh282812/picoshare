//go:build dev

package handlers

import (
	"fmt"
	"net/http"

	"github.com/mtlynch/picoshare/picoshare"
)

// devLoginPost logs the caller in as an arbitrary username, without going
// through a real identity provider. It exists only in development and e2e
// test builds (see devLoginEnabled), and shares completeLogin with the real
// OIDC callback so both paths provision accounts identically.
func (s Server) devLoginPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		username, err := picoshare.NewUsername(r.PostForm.Get("username"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid username: %v", err), http.StatusBadRequest)
			return
		}
		subject, err := picoshare.NewOIDCSubject("dev|" + username.String())
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid username: %v", err), http.StatusBadRequest)
			return
		}

		identity := picoshare.UserIdentity{
			Subject:  subject,
			Username: username,
		}

		if err := s.completeLogin(w, r, identity); err != nil {
			http.Error(w, fmt.Sprintf("Failed to complete login: %v", err), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
