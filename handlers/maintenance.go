package handlers

import (
	"log"
	"net/http"
)

// cleanupPost lets users trigger database maintenance immediately rather than
// waiting for the garbagecollect package's regular schedule.
func (s *Server) cleanupPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.collector.Collect(); err != nil {
			log.Printf("garbage collection failed: %v", err)
			http.Error(w, "Database cleanup failed", http.StatusInternalServerError)
			return
		}
	}
}
