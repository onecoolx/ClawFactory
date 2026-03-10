package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) heartbeatHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	var req model.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("heartbeat: invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := s.Registry.Heartbeat(req.AgentID); err != nil {
		logger.Warn("heartbeat: agent not found", "agent_id", req.AgentID, "error", err)
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	logger.Debug("heartbeat received", "agent_id", req.AgentID)
	writeJSON(w, http.StatusOK, model.HeartbeatResponse{Status: "ok"})
}
