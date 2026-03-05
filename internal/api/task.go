package api

import (
	"encoding/json"
	"net/http"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/go-chi/chi/v5"
)

func (s *Server) pullTaskHandler(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent_id is required")
		return
	}
	agent, err := s.Registry.GetAgent(agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
		return
	}
	task, err := s.Scheduler.AssignTask(agentID, agent.Capabilities)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if task == nil {
		writeJSON(w, http.StatusOK, model.TaskResponse{Assigned: false})
		return
	}
	writeJSON(w, http.StatusOK, model.TaskResponse{
		TaskID: task.TaskID, WorkflowID: task.WorkflowID, Type: task.Type,
		Capabilities: task.Capabilities, Input: task.Input, Assigned: true,
	})
}

func (s *Server) updateTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "taskID")
	var req model.TaskStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}
	if err := s.Queue.UpdateStatus(taskID, req.Status, req.Output, req.Error); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	// Trigger workflow engine callback
	if req.Status == "completed" {
		s.Workflow.OnTaskCompleted(taskID)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
