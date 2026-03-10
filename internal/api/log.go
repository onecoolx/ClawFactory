package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) logHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	var entry model.LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		logger.Warn("log: invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := s.Store.SaveLog(entry); err != nil {
		logger.Error("log: failed to save log entry", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
