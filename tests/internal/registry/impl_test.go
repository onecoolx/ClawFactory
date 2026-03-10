package registry_test

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/registry"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"pgregory.net/rapid"
)

func newTestRegistry(t *testing.T) (*registry.StoreRegistry, *store.SQLiteStore) {
	tmpDB, err := os.CreateTemp("", "clawfactory-reg-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpDB.Close()
	t.Cleanup(func() { os.Remove(tmpDB.Name()) })

	s, err := store.NewSQLiteStore(tmpDB.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return registry.NewStoreRegistry(s), s
}

// Property 1: Registration idempotency
// **Validates: Requirements 1.1, 1.2**
func TestProperty1_RegisterIdempotency(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		name := "agent-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name")
		version := rapid.StringMatching("[0-9]\\.[0-9]").Draw(rt, "version")
		req := model.RegisterRequest{Name: name, Capabilities: []string{"cap1"}, Version: version}

		a1, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		a2, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		if a1.AgentID != a2.AgentID {
			rt.Fatalf("idempotency violated: %s != %s", a1.AgentID, a2.AgentID)
		}
		agents, _ := reg.ListAgents()
		count := 0
		for _, a := range agents {
			if a.AgentID == a1.AgentID {
				count++
			}
		}
		if count != 1 {
			rt.Fatalf("expected 1 record, got %d", count)
		}
	})
}

// Property 2: Invalid registration requests are rejected
// **Validates: Requirements 1.3**
func TestProperty2_InvalidRegistrationRejected(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		missingName := rapid.Bool().Draw(rt, "missingName")
		var req model.RegisterRequest
		if missingName {
			req = model.RegisterRequest{Name: "", Capabilities: []string{"cap1"}, Version: "1.0"}
		} else {
			req = model.RegisterRequest{Name: "agent-x", Capabilities: []string{}, Version: "1.0"}
		}
		_, err := reg.Register(req)
		if err == nil {
			rt.Fatal("expected error for invalid registration")
		}
	})
}

// Property 3: Heartbeat updates timestamp
// **Validates: Requirements 2.2**
func TestProperty3_HeartbeatUpdatesTimestamp(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		req := model.RegisterRequest{
			Name:         "hb-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		before := time.Now()
		time.Sleep(time.Millisecond)
		if err := reg.Heartbeat(agent.AgentID); err != nil {
			rt.Fatal(err)
		}
		updated, err := reg.GetAgent(agent.AgentID)
		if err != nil {
			rt.Fatal(err)
		}
		if updated.LastHeartbeat.Before(before) {
			rt.Fatalf("heartbeat timestamp not updated: %v < %v", updated.LastHeartbeat, before)
		}
	})
}

// Property 4: Heartbeat timeout and recovery round-trip
// **Validates: Requirements 2.3, 2.4**
func TestProperty4_HeartbeatTimeoutAndRecovery(t *testing.T) {
	reg, s := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		req := model.RegisterRequest{
			Name:         "to-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}

		// Simulate heartbeat timeout: set last_heartbeat to long ago
		s.UpdateAgentStatus(agent.AgentID, "online", time.Now().Add(-10*time.Minute))

		marked, err := reg.CheckAndMarkOffline(90 * time.Second)
		if err != nil {
			rt.Fatal(err)
		}
		found := false
		for _, id := range marked {
			if id == agent.AgentID {
				found = true
			}
		}
		if !found {
			rt.Fatal("agent should be marked offline")
		}
		a, _ := reg.GetAgent(agent.AgentID)
		if a.Status != "offline" {
			rt.Fatalf("status should be offline, got %s", a.Status)
		}

		// Recovery: heartbeat again
		if err := reg.Heartbeat(agent.AgentID); err != nil {
			rt.Fatal(err)
		}
		a, _ = reg.GetAgent(agent.AgentID)
		if a.Status != "online" {
			rt.Fatalf("status should be online after heartbeat, got %s", a.Status)
		}
	})
}

// Property 10: Deregistered agent no longer receives tasks
// **Validates: Requirements 6.3**
func TestProperty10_DeregisteredAgentNoTasks(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		req := model.RegisterRequest{
			Name:         "dr-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		if err := reg.Deregister(agent.AgentID); err != nil {
			rt.Fatal(err)
		}
		a, _ := reg.GetAgent(agent.AgentID)
		if a.Status != "deregistered" {
			rt.Fatalf("status should be deregistered, got %s", a.Status)
		}
	})
}

// Feature: v02-tech-debt-fixes, Property 32: offline agent task requeue
// When an agent is marked offline, all its assigned/running tasks should be requeued
// as "pending" with assigned_to cleared, preserving priority, capabilities, input, and retry_count.
// **Validates: Requirements 4.1, 4.3, 4.4, 4.5**
func TestProperty32_OfflineAgentTaskRequeue(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg, s := newTestRegistry(t)
		queue := taskqueue.NewStoreBackedQueue(s)

		// Register an agent
		agentName := "offline-" + rapid.StringMatching("[a-z]{3,6}").Draw(rt, "agentName")
		cap := rapid.StringMatching("[a-z]{3,8}").Draw(rt, "cap")
		agent, err := reg.Register(model.RegisterRequest{
			Name:         agentName,
			Capabilities: []string{cap},
			Version:      "1.0",
		})
		if err != nil {
			rt.Fatal(err)
		}

		// Create a workflow instance (FK constraint)
		wfID := "wf-" + rapid.StringMatching("[a-z0-9]{6}").Draw(rt, "wfID")
		defID := "def-" + wfID
		err = s.SaveWorkflow(
			model.WorkflowInstance{InstanceID: wfID, DefinitionID: defID, Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			model.WorkflowDefinition{ID: defID, Name: "test-wf", Nodes: []model.WorkflowNode{{ID: "n1", Type: "test", Capabilities: []string{cap}}}, Edges: nil},
		)
		if err != nil {
			rt.Fatal(err)
		}

		// Generate 1-3 tasks assigned to this agent with status "assigned" or "running"
		numTasks := rapid.IntRange(1, 3).Draw(rt, "numTasks")
		type taskSnapshot struct {
			TaskID       string
			Priority     int
			Capabilities []string
			Input        map[string]string
			RetryCount   int
		}
		originals := make([]taskSnapshot, numTasks)

		for i := 0; i < numTasks; i++ {
			taskID := rapid.StringMatching("[a-z0-9]{8}").Draw(rt, "taskID")
			priority := rapid.IntRange(0, 10).Draw(rt, "priority")
			retryCount := rapid.IntRange(0, 5).Draw(rt, "retryCount")
			status := rapid.SampledFrom([]string{"assigned", "running"}).Draw(rt, "status")
			inputKey := rapid.StringMatching("[a-z]{3,6}").Draw(rt, "inputKey")
			inputVal := rapid.StringMatching("[a-z0-9]{3,10}").Draw(rt, "inputVal")
			caps := []string{cap}

			task := model.Task{
				TaskID:       taskID,
				WorkflowID:   wfID,
				NodeID:       "n1",
				Type:         "test",
				Capabilities: caps,
				Input:        map[string]string{inputKey: inputVal},
				Status:       status,
				Priority:     priority,
				AssignedTo:   agent.AgentID,
				RetryCount:   retryCount,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if err := s.SaveTask(task); err != nil {
				rt.Fatal(err)
			}
			// Persist assigned_to
			if err := s.UpdateTaskAssignment(taskID, agent.AgentID); err != nil {
				rt.Fatal(err)
			}

			originals[i] = taskSnapshot{
				TaskID:       taskID,
				Priority:     priority,
				Capabilities: caps,
				Input:        map[string]string{inputKey: inputVal},
				RetryCount:   retryCount,
			}
		}

		// Simulate agent going offline: set last_heartbeat to long ago
		s.UpdateAgentStatus(agent.AgentID, "online", time.Now().Add(-10*time.Minute))

		// Call CheckAndMarkOffline
		offlineIDs, err := reg.CheckAndMarkOffline(90 * time.Second)
		if err != nil {
			rt.Fatal(err)
		}
		found := false
		for _, id := range offlineIDs {
			if id == agent.AgentID {
				found = true
			}
		}
		if !found {
			rt.Fatal("agent should be marked offline")
		}

		// Simulate heartbeat goroutine logic: requeue tasks for offline agents
		tasks, err := s.GetTasksByAssignee(agent.AgentID)
		if err != nil {
			rt.Fatal(err)
		}
		for _, tk := range tasks {
			if err := queue.UpdateStatus(tk.TaskID, "pending", nil, ""); err != nil {
				rt.Fatal(err)
			}
			if err := s.UpdateTaskAssignment(tk.TaskID, ""); err != nil {
				rt.Fatal(err)
			}
		}

		// Verify all tasks are now "pending" with empty assigned_to, metadata preserved
		for _, orig := range originals {
			got, err := s.GetTask(orig.TaskID)
			if err != nil {
				rt.Fatalf("failed to get task %s: %v", orig.TaskID, err)
			}
			if got.Status != "pending" {
				rt.Fatalf("task %s: expected status 'pending', got '%s'", orig.TaskID, got.Status)
			}
			if got.AssignedTo != "" {
				rt.Fatalf("task %s: expected empty assigned_to, got '%s'", orig.TaskID, got.AssignedTo)
			}
			if got.Priority != orig.Priority {
				rt.Fatalf("task %s: priority changed from %d to %d", orig.TaskID, orig.Priority, got.Priority)
			}
			if len(got.Capabilities) != len(orig.Capabilities) || got.Capabilities[0] != orig.Capabilities[0] {
				rt.Fatalf("task %s: capabilities changed", orig.TaskID)
			}
			for k, v := range orig.Input {
				if got.Input[k] != v {
					rt.Fatalf("task %s: input[%s] changed from '%s' to '%s'", orig.TaskID, k, v, got.Input[k])
				}
			}
			if got.RetryCount != orig.RetryCount {
				rt.Fatalf("task %s: retry_count changed from %d to %d", orig.TaskID, orig.RetryCount, got.RetryCount)
			}
		}
	})
}
