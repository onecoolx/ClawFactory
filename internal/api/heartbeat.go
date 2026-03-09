package api

import (
	"encoding/json"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	var req model.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := s.Registry.Heartbeat(req.AgentID); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.HeartbeatResponse{Status: "ok"})
}
