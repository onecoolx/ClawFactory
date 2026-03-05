package scheduler

import (
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
)

// StoreScheduler is the StateStore-based scheduler implementation.
type StoreScheduler struct {
	store store.StateStore
	queue taskqueue.TaskQueue
}

// NewStoreScheduler creates a new scheduler.
func NewStoreScheduler(s store.StateStore, q taskqueue.TaskQueue) *StoreScheduler {
	return &StoreScheduler{store: s, queue: q}
}

// AssignTask assigns a matching task to the specified agent.
func (s *StoreScheduler) AssignTask(agentID string, capabilities []string) (*model.Task, error) {
	// Verify agent status
	agent, err := s.store.GetAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}
	if agent.Status != "online" {
		return nil, nil // non-online agents do not receive tasks
	}

	// Dequeue from queue by capability match
	task, err := s.queue.Dequeue(capabilities)
	if err != nil {
		return nil, fmt.Errorf("dequeue: %w", err)
	}
	if task == nil {
		return nil, nil // no matching task
	}

	// Update task status to assigned
	task.Status = "assigned"
	task.AssignedTo = agentID
	task.UpdatedAt = time.Now()
	if err := s.queue.UpdateStatus(task.TaskID, "assigned", nil, ""); err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	task.Status = "assigned"
	task.AssignedTo = agentID
	return task, nil
}

// RequeueTask requeues a task (status back to pending).
func (s *StoreScheduler) RequeueTask(taskID string) error {
	return s.queue.UpdateStatus(taskID, "pending", nil, "")
}
