package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/go-chi/chi/v5"
)
func (s *Server) listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	agents, err := s.Registry.ListAgents()
	if err != nil {
		logger.Error("list agents: failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) deregisterAgentHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	agentID := chi.URLParam(r, "agentID")
	if err := s.Registry.Deregister(agentID); err != nil {
		logger.Warn("deregister agent: not found", "agent_id", agentID, "error", err)
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	logger.Info("agent deregistered", "agent_id", agentID)

	// Publish agent deregistered event
	if s.Events != nil {
		detail, _ := json.Marshal(map[string]string{})
		if err := s.Events.Publish(model.Event{
			EventID:    generateUUID(),
			EventType:  model.EventAgentDeregistered,
			EntityType: "agent",
			EntityID:   agentID,
			Detail:     string(detail),
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			logger.Warn("failed to publish event", "event_type", model.EventAgentDeregistered, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) submitWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	var def model.WorkflowDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		logger.Warn("submit workflow: invalid request body", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	inst, err := s.Workflow.SubmitWorkflow(def)
	if err != nil {
		logger.Warn("submit workflow: validation failed", "error", err)
		writeError(w, http.StatusUnprocessableEntity, "INVALID_WORKFLOW", err.Error())
		return
	}
	logger.Info("workflow submitted", "workflow_id", inst.InstanceID)

	// Publish workflow submitted event
	if s.Events != nil {
		detail, _ := json.Marshal(map[string]string{"name": def.Name, "definition_id": def.ID})
		if err := s.Events.Publish(model.Event{
			EventID:    generateUUID(),
			EventType:  model.EventWorkflowSubmitted,
			EntityType: "workflow",
			EntityID:   inst.InstanceID,
			Detail:     string(detail),
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			logger.Warn("failed to publish event", "event_type", model.EventWorkflowSubmitted, "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, inst)
}

func (s *Server) getWorkflowStatusHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	workflowID := chi.URLParam(r, "workflowID")
	inst, err := s.Workflow.GetWorkflowStatus(workflowID)
	if err != nil {
		logger.Warn("get workflow status: not found", "workflow_id", workflowID, "error", err)
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

func (s *Server) getWorkflowArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	workflowID := chi.URLParam(r, "workflowID")
	artifacts, err := s.Memory.GetArtifacts(workflowID)
	if err != nil {
		logger.Error("get workflow artifacts: failed", "workflow_id", workflowID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, artifacts)
}

func (s *Server) getAgentLogsHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	agentID := chi.URLParam(r, "agentID")
	logs, err := s.Store.GetLogs(agentID, "", time.Time{}, time.Time{})
	if err != nil {
		logger.Error("get agent logs: failed", "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
