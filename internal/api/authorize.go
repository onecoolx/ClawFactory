package api

import (
	"encoding/json"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	var req model.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	resp := s.Policy.Authorize(req)
	status := http.StatusOK
	if !resp.Allowed {
		status = http.StatusForbidden
	}
	writeJSON(w, status, resp)
}
