package api

import (
	"encoding/json"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) logHandler(w http.ResponseWriter, r *http.Request) {
	var entry model.LogEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := s.Store.SaveLog(entry); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
