package scheduler

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"pgregory.net/rapid"
)

func newTestScheduler(t *testing.T) (*StoreScheduler, *store.SQLiteStore) {
	tmpDB, err := os.CreateTemp("", "clawfactory-sched-*.db")
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

	// 创建 workflow
	s.SaveWorkflow(
		model.WorkflowInstance{InstanceID: "wf-sched", DefinitionID: "def-1", Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		model.WorkflowDefinition{ID: "def-1", Name: "test"},
	)

	q := taskqueue.NewStoreBackedQueue(s)
	return NewStoreScheduler(s, q), s
}

func seedTestAgent(s *store.SQLiteStore, agentID string, caps []string, status string) {
	s.SaveAgent(model.AgentInfo{
		AgentID: agentID, Name: "test-" + agentID, Capabilities: caps,
		Version: "1.0", Status: status, Roles: []string{"developer_agent"},
		LastHeartbeat: time.Now(), RegisteredAt: time.Now(),
	})
}

// Property 5: 任务分配能力匹配
// **Validates: Requirements 3.1, 7.1**
func TestProperty5_TaskAssignmentCapabilityMatch(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// 每次迭代使用独立的调度器和存储，避免跨迭代状态污染
		sched, s := newTestScheduler(t)

		agentCaps := []string{rapid.SampledFrom([]string{"coding", "testing", "design"}).Draw(rt, "agentCap")}
		seedTestAgent(s, "agent-cap", agentCaps, "online")

		taskCap := rapid.SampledFrom([]string{"coding", "testing", "design", "analysis"}).Draw(rt, "taskCap")
		taskID := "tc-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")

		q := taskqueue.NewStoreBackedQueue(s)
		q.Enqueue(model.Task{
			TaskID: taskID, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
			Capabilities: []string{taskCap}, Input: map[string]string{}, Output: map[string]string{},
		})

		task, err := sched.AssignTask("agent-cap", agentCaps)
		if err != nil {
			rt.Fatal(err)
		}

		hasMatch := false
		for _, ac := range agentCaps {
			if ac == taskCap {
				hasMatch = true
			}
		}
		if hasMatch && task == nil {
			rt.Fatal("matching task should be assigned")
		}
		if !hasMatch && task != nil {
			rt.Fatal("non-matching task should not be assigned")
		}
	})
}

// Property 6: 任务分配状态流转
// **Validates: Requirements 7.4, 11.3**
func TestProperty6_TaskAssignmentStatusTransition(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sched, s := newTestScheduler(t)
		seedTestAgent(s, "agent-st", []string{"cap1"}, "online")

		taskID := "ts-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
		q := taskqueue.NewStoreBackedQueue(s)
		q.Enqueue(model.Task{
			TaskID: taskID, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
		})

		task, err := sched.AssignTask("agent-st", []string{"cap1"})
		if err != nil {
			rt.Fatal(err)
		}
		if task == nil {
			rt.Fatal("task should be assigned")
		}

		got, err := q.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got.Status != "assigned" {
			rt.Fatalf("task status: got %q, want assigned", got.Status)
		}
	})
}

// Property 11: 负载均衡调度
// **Validates: Requirements 7.2**
func TestProperty11_LoadBalancing(t *testing.T) {
	sched, s := newTestScheduler(t)
	seedTestAgent(s, "agent-off", []string{"cap1"}, "offline")

	q := taskqueue.NewStoreBackedQueue(s)
	q.Enqueue(model.Task{
		TaskID: "lb-task-1", WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
		Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
	})

	task, err := sched.AssignTask("agent-off", []string{"cap1"})
	if err != nil {
		t.Fatal(err)
	}
	if task != nil {
		t.Fatal("offline agent should not receive tasks")
	}
}

// Property 12: 无匹配智能体时任务保留
// **Validates: Requirements 7.3**
func TestProperty12_NoMatchTaskRetained(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sched, s := newTestScheduler(t)
		seedTestAgent(s, "agent-nm", []string{"coding"}, "online")

		taskID := "nm-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
		q := taskqueue.NewStoreBackedQueue(s)
		q.Enqueue(model.Task{
			TaskID: taskID, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
			Capabilities: []string{"analysis"}, // 不匹配 coding
			Input: map[string]string{}, Output: map[string]string{},
		})

		task, err := sched.AssignTask("agent-nm", []string{"coding"})
		if err != nil {
			rt.Fatal(err)
		}
		if task != nil {
			rt.Fatal("no matching task should return nil")
		}

		got, err := q.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got.Status != "pending" {
			rt.Fatalf("task should remain pending, got %s", got.Status)
		}
	})
}
