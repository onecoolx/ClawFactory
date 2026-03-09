package api

import (
	"encoding/json"
	"fmt"
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

	// On task failure, check if auto-retry is needed
	if req.Status == "failed" {
		shouldRetry, err := s.Policy.ShouldRetry(taskID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("retry check failed: %v", err))
			return
		}
		if shouldRetry {
			// Increment retry count
			if err := s.Store.IncrementTaskRetryCount(taskID); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("increment retry count failed: %v", err))
				return
			}
			// Requeue as pending
			if err := s.Queue.UpdateStatus(taskID, "pending", nil, ""); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("requeue failed: %v", err))
				return
			}
			// Clear task assignment
			if err := s.Store.UpdateTaskAssignment(taskID, ""); err != nil {
				writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("clear assignment failed: %v", err))
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
			return
		}
		// No retry, mark as permanently failed
		if err := s.Queue.UpdateStatus(taskID, "failed", req.Output, req.Error); err != nil {
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		s.Workflow.OnTaskPermanentlyFailed(taskID)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Non-failed status, keep original logic
	if err := s.Queue.UpdateStatus(taskID, req.Status, req.Output, req.Error); err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if req.Status == "completed" {
		s.Workflow.OnTaskCompleted(taskID)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
