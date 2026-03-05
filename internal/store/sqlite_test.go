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

// Property 7: 任务状态持久化往返
// **Validates: Requirements 3.5**
func TestProperty7_TaskStatusRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := newTestStore(t)

		// 先创建 workflow instance（外键约束）
		wfID := rapid.StringMatching(`^wf-[a-z0-9]{4}$`).Draw(t, "workflowID")
		inst := model.WorkflowInstance{
			InstanceID: wfID, DefinitionID: "def-1", Status: "running",
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
		if err := s.SaveWorkflow(inst, def); err != nil {
			t.Fatal(err)
		}

		taskID := rapid.StringMatching(`^task-[a-z0-9]{4}$`).Draw(t, "taskID")
		status := rapid.SampledFrom([]string{"completed", "failed"}).Draw(t, "status")
		errMsg := ""
		if status == "failed" {
			errMsg = rapid.StringMatching(`^err-[a-z]{3}$`).Draw(t, "errMsg")
		}
		output := map[string]string{"result": rapid.StringMatching(`^[a-z]{3,8}$`).Draw(t, "output")}

		task := model.Task{
			TaskID: taskID, WorkflowID: wfID, NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{},
			Output: map[string]string{}, Status: "pending", Priority: 0,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		if err := s.SaveTask(task); err != nil {
			t.Fatal(err)
		}
		if err := s.UpdateTaskStatus(taskID, status, output, errMsg); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetTask(taskID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != status {
			t.Fatalf("status: got %q, want %q", got.Status, status)
		}
		if got.Output["result"] != output["result"] {
			t.Fatalf("output mismatch: got %v, want %v", got.Output, output)
		}
		if got.Error != errMsg {
			t.Fatalf("error: got %q, want %q", got.Error, errMsg)
		}
	})
}

// Property 9: 日志存储与过滤
// **Validates: Requirements 5.2, 5.3**
func TestProperty9_LogStorageAndFiltering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := newTestStore(t)

		agentID := rapid.SampledFrom([]string{"agent-a", "agent-b"}).Draw(t, "filterAgent")
		n := rapid.IntRange(1, 10).Draw(t, "logCount")

		now := time.Now()
		for i := 0; i < n; i++ {
			aid := rapid.SampledFrom([]string{"agent-a", "agent-b"}).Draw(t, "agentID")
			entry := model.LogEntry{
				AgentID:   aid,
				TaskID:    "task-1",
				Level:     "info",
				Message:   "msg",
				Timestamp: now.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
			}
			if err := s.SaveLog(entry); err != nil {
				t.Fatal(err)
			}
		}

		logs, err := s.GetLogs(agentID, "", time.Time{}, time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range logs {
			if l.AgentID != agentID {
				t.Fatalf("log agent_id %q does not match filter %q", l.AgentID, agentID)
			}
		}
	})
}

// Property 19: 按 workflow_id 查询任务完整性
// **Validates: Requirements 13.4**
func TestProperty19_TasksByWorkflowCompleteness(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		s := newTestStore(t)

		// 创建两个 workflow
		for _, wfID := range []string{"wf-aaa", "wf-bbb"} {
			inst := model.WorkflowInstance{
				InstanceID: wfID, DefinitionID: "def-1", Status: "running",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}
			def := model.WorkflowDefinition{ID: "def-1", Name: "test"}
			s.SaveWorkflow(inst, def)
		}

		nA := rapid.IntRange(1, 5).Draw(t, "tasksA")
		nB := rapid.IntRange(1, 5).Draw(t, "tasksB")

		for i := 0; i < nA; i++ {
			s.SaveTask(model.Task{
				TaskID: rapid.StringMatching(`^ta-[a-z0-9]{4}$`).Draw(t, "taskA"),
				WorkflowID: "wf-aaa", NodeID: "n1", Type: "t", Capabilities: []string{"c"},
				Input: map[string]string{}, Output: map[string]string{}, Status: "pending",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
		}
		for i := 0; i < nB; i++ {
			s.SaveTask(model.Task{
				TaskID: rapid.StringMatching(`^tb-[a-z0-9]{4}$`).Draw(t, "taskB"),
				WorkflowID: "wf-bbb", NodeID: "n1", Type: "t", Capabilities: []string{"c"},
				Input: map[string]string{}, Output: map[string]string{}, Status: "pending",
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
		}

		tasksA, err := s.GetTasksByWorkflow("wf-aaa")
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range tasksA {
			if tk.WorkflowID != "wf-aaa" {
				t.Fatalf("task %s has workflow %s, want wf-aaa", tk.TaskID, tk.WorkflowID)
			}
		}
		if len(tasksA) != nA {
			t.Fatalf("expected %d tasks for wf-aaa, got %d", nA, len(tasksA))
		}
	})
}
