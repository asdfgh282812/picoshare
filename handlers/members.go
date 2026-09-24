package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

// membersGet serves the administrator page for reviewing every user who has
// signed in and promoting or demoting administrators.
func (s Server) membersGet() http.HandlerFunc {
	t := parseTemplates("templates/pages/members.html")

	return func(w http.ResponseWriter, r *http.Request) {
		users, err := s.store.GetUsers()
		if err != nil {
			log.Printf("failed to retrieve users: %v", err)
			http.Error(w, "Failed to retrieve users", http.StatusInternalServerError)
			return
		}

		viewer, _ := currentUser(r.Context())

		renderTemplate(w, t, struct {
			commonProps
			Users         []picoshare.User
			CurrentUserID string
		}{
			commonProps:   makeCommonProps("title.members", r.Context()),
			Users:         users,
			CurrentUserID: viewer.ID.String(),
		})
	}
}

func (s Server) memberGrantAdminPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.UserIDFromString(mux.Vars(r)["id"])
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid user ID: %v", err), http.StatusBadRequest)
			return
		}

		if err := s.store.GrantAdmin(id); err != nil {
			respondToMemberStoreError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// memberRevokeAdminPut demotes a user. It refuses to let an administrator
// revoke their own access, since doing so through the API they're currently
// using would lock them out of the page that could undo it.
func (s Server) memberRevokeAdminPut() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := picoshare.UserIDFromString(mux.Vars(r)["id"])
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid user ID: %v", err), http.StatusBadRequest)
			return
		}

		viewer, _ := currentUser(r.Context())
		if id == viewer.ID {
			http.Error(w, "You cannot revoke your own administrator access", http.StatusBadRequest)
			return
		}

		if err := s.store.RevokeAdmin(id); err != nil {
			respondToMemberStoreError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func respondToMemberStoreError(w http.ResponseWriter, err error) {
	if _, ok := errors.AsType[store.UserNotFoundError](err); ok {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if _, ok := errors.AsType[store.LastAdminError](err); ok {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("failed to change administrator status: %v", err)
	http.Error(w, "Failed to change administrator status", http.StatusInternalServerError)
}
