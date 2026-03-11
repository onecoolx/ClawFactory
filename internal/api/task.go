package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/go-chi/chi/v5"
)

func (s *Server) pullTaskHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		logger.Warn("pull task: missing agent_id")
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "agent_id is required")
		return
	}
	agent, err := s.Registry.GetAgent(agentID)
	if err != nil {
		logger.Warn("pull task: agent not found", "agent_id", agentID, "error", err)
		writeError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
		return
	}
	task, err := s.Scheduler.AssignTask(agentID, agent.Capabilities)
	if err != nil {
		logger.Error("pull task: assignment failed", "agent_id", agentID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if task == nil {
		logger.Debug("pull task: no task available", "agent_id", agentID)
		writeJSON(w, http.StatusOK, model.TaskResponse{Assigned: false})
		return
	}
	logger.Info("task assigned", "agent_id", agentID, "task_id", task.TaskID)

	// Record task assigned metric and scheduling duration
	if s.Metrics != nil {
		s.Metrics.IncTaskTotal("assigned")
		s.Metrics.ObserveSchedulingDuration(time.Since(task.CreatedAt).Seconds())
	}

	// Publish task assigned event
	if s.Events != nil {
		detail, _ := json.Marshal(map[string]string{"agent_id": agentID, "workflow_id": task.WorkflowID, "type": task.Type})
		if err := s.Events.Publish(model.Event{
			EventID:    generateUUID(),
			EventType:  model.EventTaskAssigned,
			EntityType: "task",
			EntityID:   task.TaskID,
			Detail:     string(detail),
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			logger.Warn("failed to publish event", "event_type", model.EventTaskAssigned, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, model.TaskResponse{
		TaskID: task.TaskID, WorkflowID: task.WorkflowID, Type: task.Type,
		Capabilities: task.Capabilities, Input: task.Input, Assigned: true,
	})
}

func (s *Server) updateTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	taskID := chi.URLParam(r, "taskID")
	var req model.TaskStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Warn("update task status: invalid request body", "task_id", taskID, "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	// On task failure, check if auto-retry is needed
	if req.Status == "failed" {
		shouldRetry, err := s.Policy.ShouldRetry(taskID)
		if err != nil {
			logger.Error("update task status: retry check failed", "task_id", taskID, "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("retry check failed: %v", err))
			return
		}
		if shouldRetry {
			// Use transactional retry if SQLiteStore, otherwise fallback to individual calls
			if sqliteStore, ok := s.Store.(*store.SQLiteStore); ok {
				if err := sqliteStore.RunInTransaction(func(tx *sql.Tx) error {
					return sqliteStore.RetryTaskTx(tx, taskID)
				}); err != nil {
					logger.Error("update task status: transactional retry failed", "task_id", taskID, "error", err)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("retry failed: %v", err))
					return
				}
			} else {
				// Fallback: non-transactional retry for non-SQLiteStore implementations
				if err := s.Store.IncrementTaskRetryCount(taskID); err != nil {
					logger.Error("update task status: increment retry count failed", "task_id", taskID, "error", err)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("increment retry count failed: %v", err))
					return
				}
				if err := s.Queue.UpdateStatus(taskID, "pending", nil, ""); err != nil {
					logger.Error("update task status: requeue failed", "task_id", taskID, "error", err)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("requeue failed: %v", err))
					return
				}
				if err := s.Store.UpdateTaskAssignment(taskID, ""); err != nil {
					logger.Error("update task status: clear assignment failed", "task_id", taskID, "error", err)
					writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", fmt.Sprintf("clear assignment failed: %v", err))
					return
				}
			}
			logger.Info("task retrying", "task_id", taskID)

			// Record task requeued metric and publish event
			if s.Metrics != nil {
				s.Metrics.IncTaskTotal("pending")
			}
			if s.Events != nil {
				detail, _ := json.Marshal(map[string]string{"reason": "auto_retry"})
				if err := s.Events.Publish(model.Event{
					EventID:    generateUUID(),
					EventType:  model.EventTaskRequeued,
					EntityType: "task",
					EntityID:   taskID,
					Detail:     string(detail),
					CreatedAt:  time.Now().UTC(),
				}); err != nil {
					logger.Warn("failed to publish event", "event_type", model.EventTaskRequeued, "error", err)
				}
			}

			writeJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
			return
		}
		// No retry, mark as permanently failed
		if err := s.Queue.UpdateStatus(taskID, "failed", req.Output, req.Error); err != nil {
			logger.Warn("update task status: task not found", "task_id", taskID, "error", err)
			writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}
		logger.Info("task permanently failed", "task_id", taskID)

		// Record task failed metric and publish event
		if s.Metrics != nil {
			s.Metrics.IncTaskTotal("failed")
		}
		if s.Events != nil {
			detail, _ := json.Marshal(map[string]string{"error": req.Error})
			if err := s.Events.Publish(model.Event{
				EventID:    generateUUID(),
				EventType:  model.EventTaskFailed,
				EntityType: "task",
				EntityID:   taskID,
				Detail:     string(detail),
				CreatedAt:  time.Now().UTC(),
			}); err != nil {
				logger.Warn("failed to publish event", "event_type", model.EventTaskFailed, "error", err)
			}
		}

		s.Workflow.OnTaskPermanentlyFailed(taskID)

		// Check if workflow failed after this task's permanent failure
		if s.Events != nil {
			s.publishWorkflowTerminalEvent(logger, taskID)
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	// Non-failed status, keep original logic
	if err := s.Queue.UpdateStatus(taskID, req.Status, req.Output, req.Error); err != nil {
		logger.Warn("update task status: task not found", "task_id", taskID, "error", err)
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	if req.Status == "completed" {
		logger.Info("task completed", "task_id", taskID)

		// Record task completed metric and publish event
		if s.Metrics != nil {
			s.Metrics.IncTaskTotal("completed")
		}
		if s.Events != nil {
			detail, _ := json.Marshal(map[string]string{})
			if err := s.Events.Publish(model.Event{
				EventID:    generateUUID(),
				EventType:  model.EventTaskCompleted,
				EntityType: "task",
				EntityID:   taskID,
				Detail:     string(detail),
				CreatedAt:  time.Now().UTC(),
			}); err != nil {
				logger.Warn("failed to publish event", "event_type", model.EventTaskCompleted, "error", err)
			}
		}

		s.Workflow.OnTaskCompleted(taskID)

		// Check if workflow completed after this task
		if s.Events != nil {
			s.publishWorkflowTerminalEvent(logger, taskID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// publishWorkflowTerminalEvent checks if the task's workflow has reached a terminal state
// (completed or failed) and publishes the corresponding workflow event.
func (s *Server) publishWorkflowTerminalEvent(logger *slog.Logger, taskID string) {
	task, err := s.Store.GetTask(taskID)
	if err != nil || task.WorkflowID == "" {
		return
	}
	inst, err := s.Workflow.GetWorkflowStatus(task.WorkflowID)
	if err != nil {
		return
	}
	switch inst.Status {
	case "completed":
		if s.Metrics != nil {
			s.Metrics.ObserveWorkflowDuration(time.Since(inst.CreatedAt).Seconds())
		}
		detail, _ := json.Marshal(map[string]string{"definition_id": inst.DefinitionID})
		if err := s.Events.Publish(model.Event{
			EventID:    generateUUID(),
			EventType:  model.EventWorkflowCompleted,
			EntityType: "workflow",
			EntityID:   inst.InstanceID,
			Detail:     string(detail),
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			logger.Warn("failed to publish event", "event_type", model.EventWorkflowCompleted, "error", err)
		}
	case "failed":
		if s.Metrics != nil {
			s.Metrics.ObserveWorkflowDuration(time.Since(inst.CreatedAt).Seconds())
		}
		detail, _ := json.Marshal(map[string]string{"definition_id": inst.DefinitionID})
		if err := s.Events.Publish(model.Event{
			EventID:    generateUUID(),
			EventType:  model.EventWorkflowFailed,
			EntityType: "workflow",
			EntityID:   inst.InstanceID,
			Detail:     string(detail),
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			logger.Warn("failed to publish event", "event_type", model.EventWorkflowFailed, "error", err)
		}
	}
}
