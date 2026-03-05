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
	s := newTestStore(t)

	rapid.Check(t, func(rt *rapid.T) {
		// 先创建 workflow instance（外键约束）
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

// Property 9: 日志存储与过滤
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

// Property 19: 按 workflow_id 查询任务完整性
// **Validates: Requirements 13.4**
func TestProperty19_TasksByWorkflowCompleteness(t *testing.T) {
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

	rapid.Check(t, func(rt *rapid.T) {
		nA := rapid.IntRange(1, 5).Draw(rt, "tasksA")
		nB := rapid.IntRange(1, 5).Draw(rt, "tasksB")

		// 清理之前的任务数据（通过使用唯一前缀）
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
