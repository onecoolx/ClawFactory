package store_test

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "clawfactory-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	s, err := store.NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// Property 7: Task status persistence round-trip
// **Validates: Requirements 3.5**
func TestProperty7_TaskStatusRoundTrip(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Create workflow instance first (foreign key constraint)
		wfID := "wf-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "workflowID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			rt.Fatal(err)
		}

		taskID := "task-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
		status := rapid.SampledFrom([]string{"completed", "failed"}).Draw(rt, "status")
		errMsg := ""
		if status == "failed" {
			errMsg = "err-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "errMsg")
		}
		output := map[string]string{"result": rapid.StringMatching("[a-z]{3,8}").Draw(rt, "output")}

		task := model.Task{
			TaskID: taskID, WorkflowID: wfID, NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{},
			Output: map[string]string{}, Status: "pending", Priority: 0,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := s.SaveTask(task); err != nil {
			rt.Fatal(err)
		}
		if err := s.UpdateTaskStatus(taskID, status, output, errMsg); err != nil {
			rt.Fatal(err)
		}
		got, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got.Status != status {
			rt.Fatalf("status: got %q, want %q", got.Status, status)
		}
		if got.Output["result"] != output["result"] {
			rt.Fatalf("output mismatch: got %v, want %v", got.Output, output)
		}
		if got.Error != errMsg {
			rt.Fatalf("error: got %q, want %q", got.Error, errMsg)
		}
	})
}

// Property 9: Log storage and filtering
// **Validates: Requirements 5.2, 5.3**
func TestProperty9_LogStorageAndFiltering(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		agentID := rapid.SampledFrom([]string{"agent-a", "agent-b"}).Draw(rt, "filterAgent")
		n := rapid.IntRange(1, 10).Draw(rt, "logCount")

		now := time.Now()
		for i := 0; i < n; i++ {
			aid := rapid.SampledFrom([]string{"agent-a", "agent-b"}).Draw(rt, "agentID")
			entry := model.LogEntry{
				AgentID:   aid,
				TaskID:    "task-1",
				Level:     "info",
				Message:   "msg",
				Timestamp: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			}
			if err := s.SaveLog(entry); err != nil {
				rt.Fatal(err)
			}
		}

		logs, err := s.GetLogs(agentID, "", time.Time{}, time.Time{})
		if err != nil {
			rt.Fatal(err)
		}
		for _, l := range logs {
			if l.AgentID != agentID {
				rt.Fatalf("log agent_id %q does not match filter %q", l.AgentID, agentID)
			}
		}
	})
}

// Property 19: Task query completeness by workflow_id
// **Validates: Requirements 13.4**
func TestProperty19_TasksByWorkflowCompleteness(t *testing.T) {
	s := newTestStore(t)

	// Create two workflows
	for _, wfID := range []string{"wf-aaa", "wf-bbb"} {
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		s.SaveWorkflow(inst, def)
	}

	rapid.Check(t, func(rt *rapid.T) {
		nA := rapid.IntRange(1, 5).Draw(rt, "tasksA")
		nB := rapid.IntRange(1, 5).Draw(rt, "tasksB")

		// Clean previous task data (using unique prefix)
		prefix := rapid.StringMatching("[a-z]{3}").Draw(rt, "prefix")

		for i := 0; i < nA; i++ {
			s.SaveTask(model.Task{
				TaskID:     "ta-" + prefix + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskA"),
				WorkflowID: "wf-aaa", NodeID: "n1", Type: "t", Capabilities: []string{"c"},
				Input: map[string]string{}, Output: map[string]string{}, Status: "pending",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
		}
		for i := 0; i < nB; i++ {
			s.SaveTask(model.Task{
				TaskID:     "tb-" + prefix + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskB"),
				WorkflowID: "wf-bbb", NodeID: "n1", Type: "t", Capabilities: []string{"c"},
				Input: map[string]string{}, Output: map[string]string{}, Status: "pending",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
		}

		tasksA, err := s.GetTasksByWorkflow("wf-aaa")
		if err != nil {
			rt.Fatal(err)
		}
		for _, tk := range tasksA {
			if tk.WorkflowID != "wf-aaa" {
				rt.Fatalf("task %s has workflow %s, want wf-aaa", tk.TaskID, tk.WorkflowID)
			}
		}
		// At least nA tasks (may have more from previous iterations sharing the store)
		if len(tasksA) < nA {
			rt.Fatalf("expected at least %d tasks for wf-aaa, got %d", nA, len(tasksA))
		}
	})
}

// Property 26: ListPendingTasks correctness
// **Validates: Requirements 1.1, 1.3, 1.5**
func TestProperty26_ListPendingTasksCorrectness(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Create workflow instance (foreign key constraint)
		wfID := "wf-p26-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "wfID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			rt.Fatal(err)
		}

		// Available capability and status pools
		allCaps := []string{"cap-a", "cap-b", "cap-c", "cap-d"}
		allStatuses := []string{"pending", "assigned", "running", "completed", "failed"}

		// Generate random task set
		n := rapid.IntRange(1, 10).Draw(rt, "taskCount")
		type taskRecord struct {
			id     string
			status string
			caps   []string
			prio   int
		}
		var tasks []taskRecord
		baseTime := time.Now()

		for i := 0; i < n; i++ {
			tid := wfID + "-t-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
			status := rapid.SampledFrom(allStatuses).Draw(rt, "status")
			// Each task randomly picks 1-2 capability tags
			numCaps := rapid.IntRange(1, 2).Draw(rt, "numCaps")
			caps := make([]string, numCaps)
			for j := 0; j < numCaps; j++ {
				caps[j] = rapid.SampledFrom(allCaps).Draw(rt, "cap")
			}
			prio := rapid.IntRange(0, 10).Draw(rt, "priority")

			task := model.Task{
				TaskID: tid, WorkflowID: wfID, NodeID: "n1", Type: "test",
				Capabilities: caps, Input: map[string]string{}, Output: map[string]string{},
				Status: status, Priority: prio,
				CreatedAt: baseTime.Add(time.Duration(i) * time.Millisecond),
				UpdatedAt: time.Now(),
			}
			if err := s.SaveTask(task); err != nil {
				rt.Fatal(err)
			}
			tasks = append(tasks, taskRecord{id: tid, status: status, caps: caps, prio: prio})
		}

		// Randomly select query capabilities (1-2 tags)
		queryCapsCount := rapid.IntRange(1, 2).Draw(rt, "queryCapsCount")
		queryCaps := make([]string, queryCapsCount)
		for j := 0; j < queryCapsCount; j++ {
			queryCaps[j] = rapid.SampledFrom(allCaps).Draw(rt, "queryCap")
		}

		result, err := s.ListPendingTasks(queryCaps)
		if err != nil {
			rt.Fatal(err)
		}

		// Build query capability set
		queryCapSet := make(map[string]bool)
		for _, c := range queryCaps {
			queryCapSet[c] = true
		}

		// Compute expected task IDs from this iteration that should be returned
		expectedIDs := make(map[string]bool)
		for _, tr := range tasks {
			if tr.status != "pending" {
				continue
			}
			matched := false
			for _, c := range tr.caps {
				if queryCapSet[c] {
					matched = true
					break
				}
			}
			if matched {
				expectedIDs[tr.id] = true
			}
		}

		// Verify (1): all returned tasks have status "pending"
		// Verify (2): each returned task has at least one matching capability
		resultIDs := make(map[string]bool)
		for _, tk := range result {
			if tk.Status != "pending" {
				rt.Fatalf("returned non-pending task: %s status=%s", tk.TaskID, tk.Status)
			}
			capMatched := false
			for _, c := range tk.Capabilities {
				if queryCapSet[c] {
					capMatched = true
					break
				}
			}
			if !capMatched {
				rt.Fatalf("returned task with no matching capability: %s caps=%v queryCaps=%v", tk.TaskID, tk.Capabilities, queryCaps)
			}
			resultIDs[tk.TaskID] = true
		}

		// Verify (4): all expected pending tasks from this iteration are returned
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected task %s was not returned", id)
			}
		}

		// Verify (3): results are sorted by priority descending
		for i := 1; i < len(result); i++ {
			if result[i].Priority > result[i-1].Priority {
				rt.Fatalf("priority order violation: task[%d].priority=%d > task[%d].priority=%d",
					i, result[i].Priority, i-1, result[i-1].Priority)
			}
		}
	})
}

// Property 27: ListUnfinishedTasks correctness
// **Validates: Requirements 1.2, 1.4**
func TestProperty27_ListUnfinishedTasksCorrectness(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Create workflow instance (foreign key constraint)
		wfID := "wf-p27-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "wfID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			rt.Fatal(err)
		}

		allStatuses := []string{"pending", "assigned", "running", "completed", "failed"}
		unfinishedSet := map[string]bool{"pending": true, "assigned": true, "running": true}

		// Generate random task set
		n := rapid.IntRange(1, 10).Draw(rt, "taskCount")
		type taskRecord struct {
			id     string
			status string
			prio   int
		}
		var tasks []taskRecord

		for i := 0; i < n; i++ {
			tid := wfID + "-t-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
			status := rapid.SampledFrom(allStatuses).Draw(rt, "status")
			prio := rapid.IntRange(0, 10).Draw(rt, "priority")

			task := model.Task{
				TaskID: tid, WorkflowID: wfID, NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
				Status: status, Priority: prio,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if err := s.SaveTask(task); err != nil {
				rt.Fatal(err)
			}
			tasks = append(tasks, taskRecord{id: tid, status: status, prio: prio})
		}

		result, err := s.ListUnfinishedTasks()
		if err != nil {
			rt.Fatal(err)
		}

		// Compute expected unfinished task IDs from this iteration
		// Note: duplicate taskIDs use last-write-wins semantics (database overwrite)
		finalStatus := make(map[string]string)
		for _, tr := range tasks {
			finalStatus[tr.id] = tr.status
		}
		expectedIDs := make(map[string]bool)
		for id, status := range finalStatus {
			if unfinishedSet[status] {
				expectedIDs[id] = true
			}
		}

		// Verify (1): all returned tasks have status in {pending, assigned, running}
		resultIDs := make(map[string]bool)
		for _, tk := range result {
			if !unfinishedSet[tk.Status] {
				rt.Fatalf("returned finished task: %s status=%s", tk.TaskID, tk.Status)
			}
			resultIDs[tk.TaskID] = true
		}

		// Verify (2): all unfinished tasks from this iteration are returned
		for id := range expectedIDs {
			if !resultIDs[id] {
				rt.Fatalf("expected unfinished task %s was not returned", id)
			}
		}

		// Verify (3): results are sorted by priority descending
		for i := 1; i < len(result); i++ {
			if result[i].Priority > result[i-1].Priority {
				rt.Fatalf("priority order violation: task[%d].priority=%d > task[%d].priority=%d",
					i, result[i].Priority, i-1, result[i-1].Priority)
			}
		}
	})
}

// Property 28: CountAgentActiveTasks correctness
// **Validates: Requirements 2.1, 2.2**
func TestProperty28_CountAgentActiveTasksCorrectness(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Create workflow instance (foreign key constraint)
		wfID := "wf-p28-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "wfID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			rt.Fatal(err)
		}

		allStatuses := []string{"pending", "assigned", "running", "completed", "failed"}
		activeSet := map[string]bool{"assigned": true, "running": true}
		agents := []string{"agent-x", "agent-y", "agent-z"}

		// Generate random task set, randomly assign to different agents
		n := rapid.IntRange(1, 15).Draw(rt, "taskCount")
		// Manually count active tasks per agent
		expectedCount := make(map[string]int)

		for i := 0; i < n; i++ {
			tid := wfID + "-t-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
			status := rapid.SampledFrom(allStatuses).Draw(rt, "status")
			assignedTo := rapid.SampledFrom(agents).Draw(rt, "agent")

			task := model.Task{
				TaskID: tid, WorkflowID: wfID, NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
				Status: status, Priority: 0, AssignedTo: assignedTo,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			if err := s.SaveTask(task); err != nil {
				rt.Fatal(err)
			}
			if activeSet[status] {
				expectedCount[assignedTo]++
			}
		}

		// Query target agent and verify
		targetAgent := rapid.SampledFrom(agents).Draw(rt, "targetAgent")
		count, err := s.CountAgentActiveTasks(targetAgent)
		if err != nil {
			rt.Fatal(err)
		}

		// count may include tasks from previous iterations, but should be >= expected from this round
		if count < expectedCount[targetAgent] {
			rt.Fatalf("agent %s active task count: got %d, want >= %d", targetAgent, count, expectedCount[targetAgent])
		}
	})
}

// Property 33: Task assignment persistence round-trip
// **Validates: Requirements 5.1, 5.3, 5.4**
func TestProperty33_TaskAssignmentPersistenceRoundTrip(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Create workflow instance (foreign key constraint)
		wfID := "wf-p33-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "wfID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			rt.Fatal(err)
		}

		// Create task
		taskID := "task-p33-" + rapid.StringMatching("[a-z0-9]{6}").Draw(rt, "taskID")
		task := model.Task{
			TaskID: taskID, WorkflowID: wfID, NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{"k": "v"},
			Output: map[string]string{}, Status: "pending", Priority: 0,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := s.SaveTask(task); err != nil {
			rt.Fatal(err)
		}

		// Generate random agentID and call UpdateTaskAssignment
		agentID := "agent-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "agentID")
		if err := s.UpdateTaskAssignment(taskID, agentID); err != nil {
			rt.Fatal(err)
		}

		// Read back and verify assigned_to matches
		got, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got.AssignedTo != agentID {
			rt.Fatalf("assigned_to mismatch: got %q, want %q", got.AssignedTo, agentID)
		}

		// Update to empty string (simulate requeue clearing assignment)
		if err := s.UpdateTaskAssignment(taskID, ""); err != nil {
			rt.Fatal(err)
		}
		got2, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got2.AssignedTo != "" {
			rt.Fatalf("assigned_to should be empty after clearing: got %q", got2.AssignedTo)
		}
	})
}

// Feature: v03-observability, Property 40: Event filter correctness
// Validates: Requirements 3.6
func TestProperty40_EventFilterCorrectness(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		eventTypes := []string{"agent.registered", "agent.offline", "task.assigned", "task.completed", "workflow.submitted"}
		entityIDs := []string{"ent-aaa", "ent-bbb", "ent-ccc"}

		// Generate and save random events
		n := rapid.IntRange(3, 15).Draw(rt, "eventCount")
		type eventRecord struct {
			eventType string
			entityID  string
		}
		var records []eventRecord

		for i := 0; i < n; i++ {
			evtType := rapid.SampledFrom(eventTypes).Draw(rt, "eventType")
			entID := rapid.SampledFrom(entityIDs).Draw(rt, "entityID")
			evt := model.Event{
				EventID:    "evt-p40-" + rapid.StringMatching("[a-z0-9]{8}").Draw(rt, "eventID"),
				EventType:  evtType,
				EntityType: "test",
				EntityID:   entID,
				Detail:     "{}",
				CreatedAt:  time.Now(),
			}
			if err := s.SaveEvent(evt); err != nil {
				rt.Fatal(err)
			}
			records = append(records, eventRecord{eventType: evtType, entityID: entID})
		}

		// Test filter by event_type only
		filterType := rapid.SampledFrom(eventTypes).Draw(rt, "filterType")
		result, err := s.ListEvents(model.EventFilter{EventType: filterType})
		if err != nil {
			rt.Fatal(err)
		}
		for _, e := range result {
			if e.EventType != filterType {
				rt.Fatalf("filter by event_type %q returned event with type %q", filterType, e.EventType)
			}
		}

		// Test filter by entity_id only
		filterEntity := rapid.SampledFrom(entityIDs).Draw(rt, "filterEntity")
		result2, err := s.ListEvents(model.EventFilter{EntityID: filterEntity})
		if err != nil {
			rt.Fatal(err)
		}
		for _, e := range result2 {
			if e.EntityID != filterEntity {
				rt.Fatalf("filter by entity_id %q returned event with entity_id %q", filterEntity, e.EntityID)
			}
		}

		// Test filter by both event_type and entity_id
		result3, err := s.ListEvents(model.EventFilter{EventType: filterType, EntityID: filterEntity})
		if err != nil {
			rt.Fatal(err)
		}
		for _, e := range result3 {
			if e.EventType != filterType || e.EntityID != filterEntity {
				rt.Fatalf("filter by both: expected type=%q entity=%q, got type=%q entity=%q",
					filterType, filterEntity, e.EventType, e.EntityID)
			}
		}

		// Verify combined filter returns subset of individual filters
		if len(result3) > len(result) || len(result3) > len(result2) {
			rt.Fatalf("combined filter returned more results (%d) than individual filters (type=%d, entity=%d)",
				len(result3), len(result), len(result2))
		}
	})
}

// Feature: v03-observability, Property 42: Webhook CRUD round-trip consistency
// Validates: Requirements 4.1, 4.2, 4.3, 4.4
func TestProperty42_WebhookCRUDRoundTrip(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Generate a random webhook subscription
		webhookID := "wh-p42-" + rapid.StringMatching("[a-z0-9]{8}").Draw(rt, "webhookID")
		url := "https://example.com/" + rapid.StringMatching("[a-z]{3,8}").Draw(rt, "urlPath")
		numTypes := rapid.IntRange(1, 3).Draw(rt, "numTypes")
		allTypes := []string{"agent.registered", "agent.offline", "task.assigned", "task.completed", "workflow.submitted"}
		eventTypes := make([]string, numTypes)
		for i := 0; i < numTypes; i++ {
			eventTypes[i] = rapid.SampledFrom(allTypes).Draw(rt, "eventType")
		}

		webhook := model.WebhookSubscription{
			WebhookID:  webhookID,
			URL:        url,
			EventTypes: eventTypes,
			CreatedAt:  time.Now().Truncate(time.Second),
		}

		// SaveWebhook
		if err := s.SaveWebhook(webhook); err != nil {
			rt.Fatal(err)
		}

		// ListWebhooks should contain the saved webhook
		webhooks, err := s.ListWebhooks()
		if err != nil {
			rt.Fatal(err)
		}
		found := false
		for _, w := range webhooks {
			if w.WebhookID == webhookID {
				found = true
				if w.URL != url {
					rt.Fatalf("url mismatch: got %q, want %q", w.URL, url)
				}
				if len(w.EventTypes) != len(eventTypes) {
					rt.Fatalf("event_types length mismatch: got %d, want %d", len(w.EventTypes), len(eventTypes))
				}
				for i, et := range eventTypes {
					if w.EventTypes[i] != et {
						rt.Fatalf("event_types[%d] mismatch: got %q, want %q", i, w.EventTypes[i], et)
					}
				}
				break
			}
		}
		if !found {
			rt.Fatalf("webhook %s not found after SaveWebhook", webhookID)
		}

		// DeleteWebhook
		if err := s.DeleteWebhook(webhookID); err != nil {
			rt.Fatal(err)
		}

		// ListWebhooks should no longer contain the deleted webhook
		webhooks2, err := s.ListWebhooks()
		if err != nil {
			rt.Fatal(err)
		}
		for _, w := range webhooks2 {
			if w.WebhookID == webhookID {
				rt.Fatalf("webhook %s still found after DeleteWebhook", webhookID)
			}
		}
	})
}

// Feature: v031-reliability-cli, Property 43: RunInTransaction atomicity
// Validates: Requirements 1.1, 1.2, 1.3, 1.5
func TestProperty43_RunInTransactionAtomicity(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Create workflow instance (foreign key constraint)
		wfID := "wf-p43-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "wfID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			rt.Fatal(err)
		}

		// Generate a random task with status "assigned"
		taskID := "task-p43-" + rapid.StringMatching("[a-z0-9]{6}").Draw(rt, "taskID")
		originalStatus := "assigned"
		originalAssignedTo := "agent-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "agentID")
		task := model.Task{
			TaskID: taskID, WorkflowID: wfID, NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
			Status: originalStatus, Priority: rapid.IntRange(0, 10).Draw(rt, "priority"),
			AssignedTo: originalAssignedTo, RetryCount: 0,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := s.SaveTask(task); err != nil {
			rt.Fatal(err)
		}

		// --- Test rollback: transaction returns error, changes should NOT persist ---
		rollbackErr := fmt.Errorf("deliberate-rollback-%s", rapid.StringMatching("[a-z]{3}").Draw(rt, "errSuffix"))
		err := s.RunInTransaction(func(tx *sql.Tx) error {
			_, execErr := tx.Exec(
				`UPDATE tasks SET status = 'pending', assigned_to = '' WHERE task_id = ?`,
				taskID,
			)
			if execErr != nil {
				rt.Fatal(execErr)
			}
			return rollbackErr
		})
		if err != rollbackErr {
			rt.Fatalf("RunInTransaction should return the fn error, got %v, want %v", err, rollbackErr)
		}

		// Verify task status is unchanged after rollback
		got, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got.Status != originalStatus {
			rt.Fatalf("after rollback: status got %q, want %q", got.Status, originalStatus)
		}
		if got.AssignedTo != originalAssignedTo {
			rt.Fatalf("after rollback: assigned_to got %q, want %q", got.AssignedTo, originalAssignedTo)
		}

		// --- Test commit: transaction returns nil, changes should persist ---
		newStatus := "pending"
		err = s.RunInTransaction(func(tx *sql.Tx) error {
			_, execErr := tx.Exec(
				`UPDATE tasks SET status = ?, assigned_to = '', retry_count = retry_count + 1 WHERE task_id = ?`,
				newStatus, taskID,
			)
			return execErr
		})
		if err != nil {
			rt.Fatalf("RunInTransaction commit should succeed, got %v", err)
		}

		// Verify task status has changed after commit
		got2, err := s.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got2.Status != newStatus {
			rt.Fatalf("after commit: status got %q, want %q", got2.Status, newStatus)
		}
		if got2.AssignedTo != "" {
			rt.Fatalf("after commit: assigned_to got %q, want empty", got2.AssignedTo)
		}
		if got2.RetryCount != 1 {
			rt.Fatalf("after commit: retry_count got %d, want 1", got2.RetryCount)
		}
	})
}

// Feature: v031-reliability-cli, Property 44: ListWorkflowInstances completeness and ordering
// Validates: Requirements 3.1
func TestProperty44_ListWorkflowInstancesCompletenessAndOrdering(t *testing.T) {
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(2, 10).Draw(rt, "instanceCount")
		baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

		// Generate and insert workflow instances with distinct created_at timestamps
		insertedIDs := make(map[string]bool, n)
		for i := 0; i < n; i++ {
			instID := "wf-p44-" + rapid.StringMatching("[a-z0-9]{8}").Draw(rt, "instID")
			defID := "def-p44-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "defID")
			status := rapid.SampledFrom([]string{"running", "completed", "failed"}).Draw(rt, "status")

			// Each instance gets a distinct created_at offset by i seconds
			createdAt := baseTime.Add(time.Duration(i) * time.Second)

			inst := model.WorkflowInstance{
				InstanceID:   instID,
				DefinitionID: defID,
				Status:       status,
				CreatedAt:    createdAt,
				UpdatedAt:    createdAt,
			}
			def := model.WorkflowDefinition{ID: defID, Name: "test-p44"}
			if err := s.SaveWorkflow(inst, def); err != nil {
				rt.Fatal(err)
			}
			insertedIDs[instID] = true
		}

		// Call ListWorkflowInstances
		result, err := s.ListWorkflowInstances()
		if err != nil {
			rt.Fatal(err)
		}

		// Verify completeness: all inserted IDs appear in the result
		resultIDs := make(map[string]bool, len(result))
		for _, wi := range result {
			resultIDs[wi.InstanceID] = true
		}
		for id := range insertedIDs {
			if !resultIDs[id] {
				rt.Fatalf("inserted instance %s not found in ListWorkflowInstances result", id)
			}
		}

		// Verify ordering: created_at should be non-increasing (DESC)
		for i := 1; i < len(result); i++ {
			if result[i].CreatedAt.After(result[i-1].CreatedAt) {
				rt.Fatalf("ordering violation at index %d: created_at %v > created_at %v",
					i, result[i].CreatedAt, result[i-1].CreatedAt)
			}
		}
	})
}

// TestRequeueTaskTx_Correctness verifies that RequeueTaskTx sets status to "pending" and clears assigned_to.
// Requirements: 1.2
func TestRequeueTaskTx_Correctness(t *testing.T) {
	s := newTestStore(t)

	// Create workflow instance (foreign key constraint)
	inst := model.WorkflowInstance{
		InstanceID: "wf-requeue", DefinitionID: "def-1", Status: "running",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
	if err := s.SaveWorkflow(inst, def); err != nil {
		t.Fatal(err)
	}

	// Create a task with status "assigned" and assigned_to "agent-1"
	task := model.Task{
		TaskID: "task-requeue-1", WorkflowID: "wf-requeue", NodeID: "n1", Type: "test",
		Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
		Status: "assigned", Priority: 5, AssignedTo: "agent-1", RetryCount: 0,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.SaveTask(task); err != nil {
		t.Fatal(err)
	}

	// Call RunInTransaction with RequeueTaskTx
	err := s.RunInTransaction(func(tx *sql.Tx) error {
		return s.RequeueTaskTx(tx, "task-requeue-1")
	})
	if err != nil {
		t.Fatalf("RunInTransaction+RequeueTaskTx failed: %v", err)
	}

	// Verify status is "pending" and assigned_to is ""
	got, err := s.GetTask("task-requeue-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "pending" {
		t.Errorf("status: got %q, want %q", got.Status, "pending")
	}
	if got.AssignedTo != "" {
		t.Errorf("assigned_to: got %q, want empty string", got.AssignedTo)
	}
	// retry_count should remain unchanged
	if got.RetryCount != 0 {
		t.Errorf("retry_count: got %d, want 0 (unchanged)", got.RetryCount)
	}
}

// TestRetryTaskTx_Correctness verifies that RetryTaskTx increments retry_count, sets status to "pending", and clears assigned_to.
// Requirements: 1.1
func TestRetryTaskTx_Correctness(t *testing.T) {
	s := newTestStore(t)

	// Create workflow instance (foreign key constraint)
	inst := model.WorkflowInstance{
		InstanceID: "wf-retry", DefinitionID: "def-1", Status: "running",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
	if err := s.SaveWorkflow(inst, def); err != nil {
		t.Fatal(err)
	}

	// Create a task with status "failed" and retry_count 0
	task := model.Task{
		TaskID: "task-retry-1", WorkflowID: "wf-retry", NodeID: "n1", Type: "test",
		Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
		Status: "failed", Priority: 3, AssignedTo: "agent-2", RetryCount: 0,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.SaveTask(task); err != nil {
		t.Fatal(err)
	}

	// Call RunInTransaction with RetryTaskTx
	err := s.RunInTransaction(func(tx *sql.Tx) error {
		return s.RetryTaskTx(tx, "task-retry-1")
	})
	if err != nil {
		t.Fatalf("RunInTransaction+RetryTaskTx failed: %v", err)
	}

	// Verify retry_count is 1, status is "pending", assigned_to is ""
	got, err := s.GetTask("task-retry-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.RetryCount != 1 {
		t.Errorf("retry_count: got %d, want 1", got.RetryCount)
	}
	if got.Status != "pending" {
		t.Errorf("status: got %q, want %q", got.Status, "pending")
	}
	if got.AssignedTo != "" {
		t.Errorf("assigned_to: got %q, want empty string", got.AssignedTo)
	}

	// Call RetryTaskTx again to verify retry_count increments to 2
	err = s.RunInTransaction(func(tx *sql.Tx) error {
		return s.RetryTaskTx(tx, "task-retry-1")
	})
	if err != nil {
		t.Fatalf("second RunInTransaction+RetryTaskTx failed: %v", err)
	}

	got2, err := s.GetTask("task-retry-1")
	if err != nil {
		t.Fatal(err)
	}
	if got2.RetryCount != 2 {
		t.Errorf("retry_count after second retry: got %d, want 2", got2.RetryCount)
	}
}
