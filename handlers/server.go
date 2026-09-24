package handlers

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/mtlynch/picoshare/garbagecollect"
	"github.com/mtlynch/picoshare/picoshare"
)

type (
	// IdentityProvider drives PicoShare's OpenID Connect login flow against an
	// administrator-configured identity provider, such as a Synology SSO
	// Server.
	IdentityProvider interface {
		// Redirect sends the browser to the identity provider to begin a login.
		Redirect(w http.ResponseWriter, r *http.Request)
		// Callback validates the identity provider's response and returns the
		// identity it asserts for the user.
		Callback(w http.ResponseWriter, r *http.Request) (picoshare.UserIdentity, error)
	}

	// Params holds everything Server needs to satisfy HTTP requests.
	Params struct {
		IdentityProvider IdentityProvider
		Store            Store
		CheckSpace       SpaceCheckFunc
		Collector        *garbagecollect.Collector
		Now              NowFunc
	}

	Server struct {
		router           *mux.Router
		identityProvider IdentityProvider
		store            Store
		checkSpace       SpaceCheckFunc
		collector        *garbagecollect.Collector
		now              NowFunc
	}
)

// Router returns the underlying router interface for the server.
func (s Server) Router() *mux.Router {
	return s.router
}

// New creates a new server with all the state it needs to satisfy HTTP
// requests.
func New(p Params) Server {
	s := Server{
		router:           mux.NewRouter(),
		identityProvider: p.IdentityProvider,
		store:            p.Store,
		checkSpace:       p.CheckSpace,
		collector:        p.Collector,
		now:              p.Now,
	}

	s.routes()
	return s
}
