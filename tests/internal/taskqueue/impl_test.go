package taskqueue_test

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"pgregory.net/rapid"
)

func newTestQueue(t *testing.T) *taskqueue.StoreBackedQueue {
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

	// Create workflow instance to satisfy foreign key
	s.SaveWorkflow(
		model.WorkflowInstance{InstanceID: "wf-test", DefinitionID: "def-1", Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		model.WorkflowDefinition{ID: "def-1", Name: "test"},
	)
	return taskqueue.NewStoreBackedQueue(s)
}

// Property 16: Enqueue initial status
// **Validates: Requirements 11.2**
func TestProperty16_EnqueueInitialStatus(t *testing.T) {
	q := newTestQueue(t)

	rapid.Check(t, func(rt *rapid.T) {
		taskID := "tq-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
		task := model.Task{
			TaskID: taskID, WorkflowID: "wf-test", NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
			Status: "whatever", // should be overridden to pending on enqueue
		}
		if err := q.Enqueue(task); err != nil {
			rt.Fatal(err)
		}
		got, err := q.GetTask(taskID)
		if err != nil {
			rt.Fatal(err)
		}
		if got.Status != "pending" {
			rt.Fatalf("enqueued task status: got %q, want %q", got.Status, "pending")
		}
	})
}

// Property 17: Task priority ordering
// **Validates: Requirements 11.4**
func TestProperty17_TaskPriorityOrdering(t *testing.T) {
	q := newTestQueue(t)

	rapid.Check(t, func(rt *rapid.T) {
		// Clean up: mark all pending tasks as completed
		for {
			task, err := q.Dequeue([]string{"cap1"})
			if err != nil {
				rt.Fatal(err)
			}
			if task == nil {
				break
			}
			q.UpdateStatus(task.TaskID, "completed", nil, "")
		}

		n := rapid.IntRange(2, 6).Draw(rt, "taskCount")
		for i := 0; i < n; i++ {
			prio := rapid.IntRange(0, 100).Draw(rt, "priority")
			q.Enqueue(model.Task{
				TaskID:     "tp-" + rapid.StringMatching("[a-z0-9]{5}").Draw(rt, "taskID"),
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
				rt.Fatal(err)
			}
			if task == nil {
				break
			}
			priorities = append(priorities, task.Priority)
			q.UpdateStatus(task.TaskID, "completed", nil, "")
		}
		for i := 1; i < len(priorities); i++ {
			if priorities[i] > priorities[i-1] {
				rt.Fatalf("priority not descending: %v", priorities)
			}
		}
	})
}
