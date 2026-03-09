package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"github.com/go-chi/chi/v5"
	"pgregory.net/rapid"
)

// --- Mock implementations ---

// mockPolicyEngine implements policy.PolicyEngine.
// ShouldRetry reads the task's retry_count from the store and compares with maxRetries.
type mockPolicyEngine struct {
	store      store.StateStore
	maxRetries int
}

func (m *mockPolicyEngine) CanExecuteTask(agentID string, taskCapabilities []string) bool {
	return true
}
func (m *mockPolicyEngine) Authorize(req model.AuthorizeRequest) model.AuthorizeResponse {
	return model.AuthorizeResponse{Allowed: true}
}
func (m *mockPolicyEngine) ShouldRetry(taskID string) (bool, error) {
	task, err := m.store.GetTask(taskID)
	if err != nil {
		return false, err
	}
	return task.RetryCount < m.maxRetries, nil
}
func (m *mockPolicyEngine) GetMaxRetries() int { return m.maxRetries }
func (m *mockPolicyEngine) IsToolAllowed(agentID string, toolName string) bool {
	return true
}
func (m *mockPolicyEngine) CheckRateLimit(agentID string, toolName string) bool {
	return true
}

// mockWorkflowEngine implements workflow.WorkflowEngine.
// Tracks calls to OnTaskPermanentlyFailed for verification.
type mockWorkflowEngine struct {
	mu                     sync.Mutex
	permanentlyFailedTasks []string
}

func (m *mockWorkflowEngine) SubmitWorkflow(def model.WorkflowDefinition) (model.WorkflowInstance, error) {
	return model.WorkflowInstance{}, nil
}
func (m *mockWorkflowEngine) ValidateDAG(def model.WorkflowDefinition) error { return nil }
func (m *mockWorkflowEngine) OnTaskCompleted(taskID string) error            { return nil }
func (m *mockWorkflowEngine) OnTaskPermanentlyFailed(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permanentlyFailedTasks = append(m.permanentlyFailedTasks, taskID)
	return nil
}
func (m *mockWorkflowEngine) GetWorkflowStatus(instanceID string) (model.WorkflowInstance, error) {
	return model.WorkflowInstance{}, nil
}
func (m *mockWorkflowEngine) wasPermanentlyFailed(taskID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range m.permanentlyFailedTasks {
		if id == taskID {
			return true
		}
	}
	return false
}
func (m *mockWorkflowEngine) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.permanentlyFailedTasks = nil
}

// --- Test helpers ---

// newTestStoreForRapid creates a temp SQLite store for use inside rapid.Check.
// Returns the store and a cleanup function that must be called via defer.
func newTestStoreForRapid(rt *rapid.T) (*store.SQLiteStore, func()) {
	tmpFile, err := os.CreateTemp("", "clawfactory-api-test-*.db")
	if err != nil {
		rt.Fatal(err)
	}
	tmpFile.Close()

	s, err := store.NewSQLiteStore(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		rt.Fatal(err)
	}
	cleanup := func() {
		s.Close()
		os.Remove(tmpFile.Name())
	}
	return s, cleanup
}

// setupTestRouter creates a chi router with the updateTaskStatusHandler wired up.
func setupTestRouter(srv *Server) http.Handler {
	r := chi.NewRouter()
	r.Post("/v1/tasks/{taskID}/status", srv.updateTaskStatusHandler)
	return r
}

// sendFailedStatus sends a POST request to update task status to "failed".
func sendFailedStatus(router http.Handler, taskID string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(model.TaskStatusUpdate{Status: "failed", Error: "test error"})
	req := httptest.NewRequest("POST", fmt.Sprintf("/v1/tasks/%s/status", taskID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// ensureWorkflow creates a workflow instance for foreign key constraints.
func ensureWorkflow(rt *rapid.T, s store.StateStore, wfID string) {
	inst := model.WorkflowInstance{
		InstanceID: wfID, DefinitionID: "def-test", Status: "running",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	def := model.WorkflowDefinition{ID: "def-test", Name: "test-wf"}
	if err := s.SaveWorkflow(inst, def); err != nil {
		rt.Fatal(err)
	}
}

// Feature: v02-tech-debt-fixes, Property 30: retry vs permanent failure
// When a task status is updated to "failed": if retry_count < max_retries, the task
// should be requeued as "pending" with retry_count incremented by 1; if retry_count >= max_retries,
// the task should remain "failed".
// **Validates: Requirements 3.1, 3.3, 3.4, 3.5**
func TestProperty30_RetryVsPermanentFailure(t *testing.T) {
	const maxRetries = 3

	rapid.Check(t, func(rt *rapid.T) {
		// Fresh store per iteration to avoid cross-contamination
		s, cleanup := newTestStoreForRapid(rt)
		defer cleanup()
		queue := taskqueue.NewStoreBackedQueue(s)
		mockWf := &mockWorkflowEngine{}
		mockPolicy := &mockPolicyEngine{store: s, maxRetries: maxRetries}

		srv := &Server{
			Store:    s,
			Queue:    queue,
			Policy:   mockPolicy,
			Workflow: mockWf,
		}
		router := setupTestRouter(srv)

		// Generate a random retry_count from 0 to max_retries+2 (covers both branches)
		retryCount := rapid.IntRange(0, maxRetries+2).Draw(rt, "retryCount")

		// Create workflow for foreign key
		wfID := fmt.Sprintf("wf-p30-%d", retryCount)
		ensureWorkflow(rt, s, wfID)

		// Create and enqueue a task with the given retry_count
		taskID := fmt.Sprintf("task-p30-%s", rapid.StringMatching("[a-z0-9]{6}").Draw(rt, "taskSuffix"))
		task := model.Task{
			TaskID:       taskID,
			WorkflowID:   wfID,
			NodeID:       "node-1",
			Type:         "test",
			Capabilities: []string{"cap-a"},
			Input:        map[string]string{"key": "value"},
			Output:       map[string]string{},
			Status:       "running", // task must be in a non-pending state before failing
			Priority:     rapid.IntRange(0, 10).Draw(rt, "priority"),
			RetryCount:   retryCount,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := s.SaveTask(task); err != nil {
			rt.Fatal(err)
		}

		// Send "failed" status update
		rec := sendFailedStatus(router, taskID)
		if rec.Code != http.StatusOK {
			rt.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify the response and task state
		var resp map[string]string
		json.Unmarshal(rec.Body.Bytes(), &resp)

		// Read the task back from the store
		got, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}

		if retryCount < maxRetries {
			// Should be retried: status becomes "pending", retry_count incremented
			if resp["status"] != "retrying" {
				rt.Fatalf("retry_count=%d < max=%d: expected response status 'retrying', got %q", retryCount, maxRetries, resp["status"])
			}
			if got.Status != "pending" {
				rt.Fatalf("retry_count=%d < max=%d: expected task status 'pending', got %q", retryCount, maxRetries, got.Status)
			}
			if got.RetryCount != retryCount+1 {
				rt.Fatalf("retry_count=%d < max=%d: expected retry_count=%d, got %d", retryCount, maxRetries, retryCount+1, got.RetryCount)
			}
			if got.AssignedTo != "" {
				rt.Fatalf("retry_count=%d < max=%d: expected assigned_to cleared, got %q", retryCount, maxRetries, got.AssignedTo)
			}
			if mockWf.wasPermanentlyFailed(taskID) {
				rt.Fatalf("retry_count=%d < max=%d: OnTaskPermanentlyFailed should NOT have been called", retryCount, maxRetries)
			}
		} else {
			// Should be permanently failed
			if resp["status"] != "ok" {
				rt.Fatalf("retry_count=%d >= max=%d: expected response status 'ok', got %q", retryCount, maxRetries, resp["status"])
			}
			if got.Status != "failed" {
				rt.Fatalf("retry_count=%d >= max=%d: expected task status 'failed', got %q", retryCount, maxRetries, got.Status)
			}
			if got.RetryCount != retryCount {
				rt.Fatalf("retry_count=%d >= max=%d: retry_count should not change, got %d", retryCount, maxRetries, got.RetryCount)
			}
			if !mockWf.wasPermanentlyFailed(taskID) {
				rt.Fatalf("retry_count=%d >= max=%d: OnTaskPermanentlyFailed should have been called", retryCount, maxRetries)
			}
		}
	})
}

// Feature: v02-tech-debt-fixes, Property 31: retry preserves task metadata
// For any retried task, the task's priority, capabilities, and input data should remain
// identical after retry.
// **Validates: Requirements 3.6**
func TestProperty31_RetryPreservesTaskMetadata(t *testing.T) {
	const maxRetries = 5 // high enough so all generated tasks will be retried

	rapid.Check(t, func(rt *rapid.T) {
		s, cleanup := newTestStoreForRapid(rt)
		defer cleanup()
		queue := taskqueue.NewStoreBackedQueue(s)
		mockWf := &mockWorkflowEngine{}
		mockPolicy := &mockPolicyEngine{store: s, maxRetries: maxRetries}

		srv := &Server{
			Store:    s,
			Queue:    queue,
			Policy:   mockPolicy,
			Workflow: mockWf,
		}
		router := setupTestRouter(srv)

		// Generate random task metadata
		retryCount := rapid.IntRange(0, maxRetries-1).Draw(rt, "retryCount") // always < maxRetries so it retries

		// Generate random capabilities (1-3 items)
		numCaps := rapid.IntRange(1, 3).Draw(rt, "numCaps")
		caps := make([]string, numCaps)
		for i := 0; i < numCaps; i++ {
			caps[i] = rapid.StringMatching("[a-z_]{3,10}").Draw(rt, fmt.Sprintf("cap_%d", i))
		}

		// Generate random input data (1-3 key-value pairs)
		numInputs := rapid.IntRange(1, 3).Draw(rt, "numInputs")
		inputData := make(map[string]string)
		for i := 0; i < numInputs; i++ {
			key := rapid.StringMatching("[a-z]{2,6}").Draw(rt, fmt.Sprintf("inputKey_%d", i))
			val := rapid.StringMatching("[a-z0-9]{3,12}").Draw(rt, fmt.Sprintf("inputVal_%d", i))
			inputData[key] = val
		}

		priority := rapid.IntRange(0, 100).Draw(rt, "priority")

		// Create workflow for foreign key
		wfID := fmt.Sprintf("wf-p31-%s", rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "wfSuffix"))
		ensureWorkflow(rt, s, wfID)

		taskID := fmt.Sprintf("task-p31-%s", rapid.StringMatching("[a-z0-9]{6}").Draw(rt, "taskSuffix"))
		task := model.Task{
			TaskID:       taskID,
			WorkflowID:   wfID,
			NodeID:       "node-1",
			Type:         "test",
			Capabilities: caps,
			Input:        inputData,
			Output:       map[string]string{},
			Status:       "running",
			Priority:     priority,
			RetryCount:   retryCount,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := s.SaveTask(task); err != nil {
			rt.Fatal(err)
		}

		// Send "failed" status update — should trigger retry since retryCount < maxRetries
		rec := sendFailedStatus(router, taskID)
		if rec.Code != http.StatusOK {
			rt.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp map[string]string
		json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["status"] != "retrying" {
			rt.Fatalf("expected 'retrying' response, got %q", resp["status"])
		}

		// Read the task back and verify metadata is preserved
		got, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}

		// Priority must be unchanged
		if got.Priority != priority {
			rt.Fatalf("priority changed: expected %d, got %d", priority, got.Priority)
		}

		// Capabilities must be unchanged
		if !reflect.DeepEqual(got.Capabilities, caps) {
			rt.Fatalf("capabilities changed: expected %v, got %v", caps, got.Capabilities)
		}

		// Input data must be unchanged
		if !reflect.DeepEqual(got.Input, inputData) {
			rt.Fatalf("input data changed: expected %v, got %v", inputData, got.Input)
		}
	})
}
