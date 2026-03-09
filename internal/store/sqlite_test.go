package store

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"pgregory.net/rapid"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "clawfactory-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	s, err := NewSQLiteStore(tmpFile.Name())
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
