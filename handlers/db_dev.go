//go:build dev

package handlers

import (
	"net/http"
)

// addDevRoutes adds debug routes that we only use during development or e2e
// tests.
func (s *Server) addDevRoutes() {
	// Unlike /api/maintenance/cleanup, this route doesn't require
	// authentication, so e2e tests can trigger cleanup directly.
	s.router.HandleFunc("/api/debug/db/cleanup", s.cleanupPost()).Methods(http.MethodPost)
	s.router.HandleFunc("/api/debug/login", s.devLoginPost()).Methods(http.MethodPost)
}
