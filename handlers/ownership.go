package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/mtlynch/picoshare/picoshare"
	"github.com/mtlynch/picoshare/store"
)

// manageableEntry looks up id and confirms that the requester may view, edit,
// or delete it. A missing entry and one the requester doesn't own get the
// same 404 response, so a download link's existence isn't distinguishable
// from a stranger's entry ID guess. It writes the HTTP response itself on
// failure and returns ok=false, so callers can just return.
func (s Server) manageableEntry(w http.ResponseWriter, r *http.Request, id picoshare.EntryID) (metadata picoshare.UploadMetadata, ok bool) {
	entry, err := s.store.GetEntryMetadata(id)
	if _, notFound := errors.AsType[store.EntryNotFoundError](err); notFound {
		http.Error(w, "entry not found", http.StatusNotFound)
		return picoshare.UploadMetadata{}, false
	} else if err != nil {
		log.Printf("error retrieving entry with id %v: %v", id, err)
		http.Error(w, "failed to retrieve entry", http.StatusInternalServerError)
		return picoshare.UploadMetadata{}, false
	}

	user, _ := currentUser(r.Context())
	if !user.CanManageEntry(entry) {
		http.Error(w, "entry not found", http.StatusNotFound)
		return picoshare.UploadMetadata{}, false
	}

	return entry, true
}

// manageableGuestLink looks up id and confirms that the requester created it.
// A missing guest link and one the requester doesn't own get the same 404
// response.
func (s Server) manageableGuestLink(w http.ResponseWriter, r *http.Request, id picoshare.GuestLinkID) (gl picoshare.GuestLink, ok bool) {
	gl, err := s.store.GetGuestLink(id)
	if _, notFound := errors.AsType[store.GuestLinkNotFoundError](err); notFound {
		http.Error(w, "guest link not found", http.StatusNotFound)
		return picoshare.GuestLink{}, false
	} else if err != nil {
		log.Printf("error retrieving guest link with id %v: %v", id, err)
		http.Error(w, "failed to retrieve guest link", http.StatusInternalServerError)
		return picoshare.GuestLink{}, false
	}

	user, _ := currentUser(r.Context())
	if !user.OwnsGuestLink(gl) {
		http.Error(w, "guest link not found", http.StatusNotFound)
		return picoshare.GuestLink{}, false
	}

	return gl, true
}
