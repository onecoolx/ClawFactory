package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	var req model.AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("authorize: invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	resp := s.Policy.Authorize(req)
	status := http.StatusOK
	if !resp.Allowed {
		logger.Info("authorization denied", "agent_id", req.AgentID, "action", req.Action)
		status = http.StatusForbidden
	}
	writeJSON(w, status, resp)
}
