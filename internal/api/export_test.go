package api

import "net/http"

// TestableUpdateTaskStatusHandler exposes the unexported updateTaskStatusHandler for external test packages.
func (s *Server) TestableUpdateTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	s.updateTaskStatusHandler(w, r)
}
