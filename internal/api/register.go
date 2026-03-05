package api

import (
	"encoding/json"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	agent, err := s.Registry.Register(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.RegisterResponse{AgentID: agent.AgentID})
}
