package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
)

func (s *Server) registerHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("register: invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	agent, err := s.Registry.Register(req)
	if err != nil {
		logger.Warn("register: registration failed", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	logger.Info("agent registered", "agent_id", agent.AgentID)

	// Publish agent registered event
	if s.Events != nil {
		detail, _ := json.Marshal(map[string]string{"name": req.Name, "version": req.Version})
		if err := s.Events.Publish(model.Event{
			EventID:    generateUUID(),
			EventType:  model.EventAgentRegistered,
			EntityType: "agent",
			EntityID:   agent.AgentID,
			Detail:     string(detail),
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			logger.Warn("failed to publish event", "event_type", model.EventAgentRegistered, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, model.RegisterResponse{AgentID: agent.AgentID})
}
