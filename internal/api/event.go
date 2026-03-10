package api

import (
	"log/slog"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
)

// listEventsHandler handles GET /v1/admin/events.
// Supports optional query params: event_type, entity_id.
func (s *Server) listEventsHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	filter := model.EventFilter{
		EventType: r.URL.Query().Get("event_type"),
		EntityID:  r.URL.Query().Get("entity_id"),
	}

	events, err := s.Events.ListEvents(filter)
	if err != nil {
		logger.Error("failed to list events", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list events")
		return
	}

	logger.Info("listed events", "count", len(events), "filter_type", filter.EventType, "filter_entity", filter.EntityID)
	writeJSON(w, http.StatusOK, events)
}
