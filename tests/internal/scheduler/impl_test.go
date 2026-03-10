package scheduler_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/scheduler"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"pgregory.net/rapid"
)

func newTestScheduler(t *testing.T) (*scheduler.StoreScheduler, *store.SQLiteStore) {
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

	// Create workflow
	s.SaveWorkflow(
		model.WorkflowInstance{InstanceID: "wf-sched", DefinitionID: "def-1", Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		model.WorkflowDefinition{ID: "def-1", Name: "test"},
	)

	q := taskqueue.NewStoreBackedQueue(s)
	return scheduler.NewStoreScheduler(s, q), s
}

func seedTestAgent(s *store.SQLiteStore, agentID string, caps []string, status string) {
	s.SaveAgent(model.AgentInfo{
		AgentID: agentID, Name: "test-" + agentID, Capabilities: caps,
		Version: "1.0", Status: status, Roles: []string{"developer_agent"},
		LastHeartbeat: time.Now(), RegisteredAt: time.Now(),
	})
}

// Property 5: Task assignment capability match
// **Validates: Requirements 3.1, 7.1**
func TestProperty5_TaskAssignmentCapabilityMatch(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Use independent scheduler and store per iteration to avoid cross-iteration state pollution
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

// Property 6: Task assignment status transition
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

// Property 11: Load balancing scheduling
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

// Property 12: Task retained when no matching agent
// **Validates: Requirements 7.3**
func TestProperty12_NoMatchTaskRetained(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sched, s := newTestScheduler(t)
		seedTestAgent(s, "agent-nm", []string{"coding"}, "online")

		taskID := "nm-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "taskID")
		q := taskqueue.NewStoreBackedQueue(s)
		q.Enqueue(model.Task{
			TaskID: taskID, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
			Capabilities: []string{"analysis"}, // does not match coding
			Input:        map[string]string{}, Output: map[string]string{},
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

// Property 29: Load balancing assignment
// When the requesting agent is not the lowest-loaded matching agent, Scheduler should
// reject assignment (return nil); when the requesting agent is the lowest-loaded (or tied
// for lowest) matching agent, Scheduler should assign the task normally.
// **Validates: Requirements 2.3, 2.4, 2.5**
func TestProperty29_LoadBalancingAssignment(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		sched, s := newTestScheduler(t)

		// Two online agents, both with "cap1" capability
		seedTestAgent(s, "agent-a", []string{"cap1"}, "online")
		seedTestAgent(s, "agent-b", []string{"cap1"}, "online")

		// Assign 1-3 existing tasks to agent-a (higher load)
		loadA := rapid.IntRange(1, 3).Draw(rt, "loadA")
		for i := 0; i < loadA; i++ {
			tid := fmt.Sprintf("load-a-%d", i)
			s.SaveTask(model.Task{
				TaskID: tid, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Status: "assigned",
				Input: map[string]string{}, Output: map[string]string{},
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			})
			s.UpdateTaskAssignment(tid, "agent-a")
		}
		// agent-b has no existing tasks (load=0, strictly lower than agent-a)

		// Enqueue a new pending task
		pendingID := "pending-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "pendingID")
		q := taskqueue.NewStoreBackedQueue(s)
		q.Enqueue(model.Task{
			TaskID: pendingID, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
			Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
		})

		// agent-a (higher load) requests task -> should return nil (not lowest loaded)
		taskA, err := sched.AssignTask("agent-a", []string{"cap1"})
		if err != nil {
			rt.Fatal(err)
		}
		if taskA != nil {
			rt.Fatalf("higher-loaded agent-a (load=%d) should not receive task, but got %s", loadA, taskA.TaskID)
		}

		// agent-b (lower load) requests task -> should return task
		taskB, err := sched.AssignTask("agent-b", []string{"cap1"})
		if err != nil {
			rt.Fatal(err)
		}
		if taskB == nil {
			rt.Fatal("lowest-loaded agent-b (load=0) should receive task")
		}
		if taskB.TaskID != pendingID {
			rt.Fatalf("expected task %s, got %s", pendingID, taskB.TaskID)
		}
	})

	// When tied for lowest load, both agents should receive tasks
	t.Run("tied_load", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			sched, s := newTestScheduler(t)

			seedTestAgent(s, "agent-x", []string{"cap1"}, "online")
			seedTestAgent(s, "agent-y", []string{"cap1"}, "online")

			// Assign equal number of existing tasks to both agents (0-2)
			tiedLoad := rapid.IntRange(0, 2).Draw(rt, "tiedLoad")
			for i := 0; i < tiedLoad; i++ {
				tidX := fmt.Sprintf("tied-x-%d", i)
				s.SaveTask(model.Task{
					TaskID: tidX, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
					Capabilities: []string{"cap1"}, Status: "assigned",
					Input: map[string]string{}, Output: map[string]string{},
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				})
				s.UpdateTaskAssignment(tidX, "agent-x")

				tidY := fmt.Sprintf("tied-y-%d", i)
				s.SaveTask(model.Task{
					TaskID: tidY, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
					Capabilities: []string{"cap1"}, Status: "assigned",
					Input: map[string]string{}, Output: map[string]string{},
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				})
				s.UpdateTaskAssignment(tidY, "agent-y")
			}

			// Enqueue two pending tasks
			pid1 := "tied-p1-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "pid1")
			pid2 := "tied-p2-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "pid2")
			q := taskqueue.NewStoreBackedQueue(s)
			q.Enqueue(model.Task{
				TaskID: pid1, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
			})
			q.Enqueue(model.Task{
				TaskID: pid2, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
			})

			// With equal load, both agents should receive tasks
			taskX, err := sched.AssignTask("agent-x", []string{"cap1"})
			if err != nil {
				rt.Fatal(err)
			}
			if taskX == nil {
				rt.Fatal("tied-load agent-x should receive task")
			}

			taskY, err := sched.AssignTask("agent-y", []string{"cap1"})
			if err != nil {
				rt.Fatal(err)
			}
			if taskY == nil {
				rt.Fatal("tied-load agent-y should receive task")
			}
		})
	})

	// When only one online agent exists, load doesn't matter, should assign directly
	t.Run("single_agent", func(t *testing.T) {
		rapid.Check(t, func(rt *rapid.T) {
			sched, s := newTestScheduler(t)

			seedTestAgent(s, "agent-solo", []string{"cap1"}, "online")

			// Give agent-solo some existing load
			soloLoad := rapid.IntRange(0, 3).Draw(rt, "soloLoad")
			for i := 0; i < soloLoad; i++ {
				tid := fmt.Sprintf("solo-%d", i)
				s.SaveTask(model.Task{
					TaskID: tid, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
					Capabilities: []string{"cap1"}, Status: "assigned",
					Input: map[string]string{}, Output: map[string]string{},
					CreatedAt: time.Now(), UpdatedAt: time.Now(),
				})
				s.UpdateTaskAssignment(tid, "agent-solo")
			}

			pendingID := "solo-p-" + rapid.StringMatching("[a-z0-9]{4}").Draw(rt, "soloP")
			q := taskqueue.NewStoreBackedQueue(s)
			q.Enqueue(model.Task{
				TaskID: pendingID, WorkflowID: "wf-sched", NodeID: "n1", Type: "test",
				Capabilities: []string{"cap1"}, Input: map[string]string{}, Output: map[string]string{},
			})

			task, err := sched.AssignTask("agent-solo", []string{"cap1"})
			if err != nil {
				rt.Fatal(err)
			}
			if task == nil {
				rt.Fatalf("single online agent (load=%d) should always receive task", soloLoad)
			}
		})
	})
}
