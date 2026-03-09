package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/go-chi/chi/v5"
)

func (s *Server) listAgentsHandler(w http.ResponseWriter, r *http.Request) {
	agents, err := s.Registry.ListAgents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) deregisterAgentHandler(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	if err := s.Registry.Deregister(agentID); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) submitWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	var def model.WorkflowDefinition
	if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	inst, err := s.Workflow.SubmitWorkflow(def)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_WORKFLOW", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, inst)
}

func (s *Server) getWorkflowStatusHandler(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	inst, err := s.Workflow.GetWorkflowStatus(workflowID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, inst)
}

func (s *Server) getWorkflowArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	workflowID := chi.URLParam(r, "workflowID")
	artifacts, err := s.Memory.GetArtifacts(workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, artifacts)
}

func (s *Server) getAgentLogsHandler(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentID")
	logs, err := s.Store.GetLogs(agentID, "", time.Time{}, time.Time{})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}
