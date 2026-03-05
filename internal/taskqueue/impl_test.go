package taskqueue

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

func newTestQueue(t testing.TB) *StoreBackedQueue {
	tmpFile, err := os.CreateTemp("", "clawfactory-tq-*.db")
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

	// 创建 workflow instance 以满足外键
	s.SaveWorkflow(
		model.WorkflowInstance{InstanceID: "wf-test", DefinitionID: "def-1", Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		model.WorkflowDefinition{ID: "def-1", Name: "test"},
	)
	return NewStoreBackedQueue(s)
}

// Property 16: 任务入队初始状态
// **Validates: Requirements 11.2**
func TestProperty16_EnqueueInitialStatus(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		q := newTestQueue(t)
		taskID := rapid.StringMatching(`^tq-[a-z0-9]{4}$`).Draw(t, "taskID")
		task := model.Task{
			TaskID: taskID, WorkflowID: "wf-test", NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
			Status: "whatever", // 入队时应被覆盖为 pending
		}
		if err := q.Enqueue(task); err != nil {
			t.Fatal(err)
		}
		got, err := q.GetTask(taskID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "pending" {
			t.Fatalf("enqueued task status: got %q, want %q", got.Status, "pending")
		}
	})
}

// Property 17: 任务优先级排序
// **Validates: Requirements 11.4**
func TestProperty17_TaskPriorityOrdering(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		q := newTestQueue(t)
		n := rapid.IntRange(2, 6).Draw(t, "taskCount")
		for i := 0; i < n; i++ {
			prio := rapid.IntRange(0, 100).Draw(t, "priority")
			q.Enqueue(model.Task{
				TaskID: rapid.StringMatching(`^tp-[a-z0-9]{5}$`).Draw(t, "taskID"),
				WorkflowID: "wf-test", NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Input: map[string]string{},
				Output: map[string]string{}, Priority: prio,
			})
		}
		// Dequeue all and verify priority ordering
		var priorities []int
		for {
			task, err := q.Dequeue([]string{"cap1"})
			if err != nil {
				t.Fatal(err)
			}
			if task == nil {
				break
			}
			priorities = append(priorities, task.Priority)
			// Mark as completed so it's not dequeued again
			q.UpdateStatus(task.TaskID, "completed", nil, "")
		}
		for i := 1; i < len(priorities); i++ {
			if priorities[i] > priorities[i-1] {
				t.Fatalf("priority not descending: %v", priorities)
			}
		}
	})
}
