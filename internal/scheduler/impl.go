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
// Implements load balancing: if the requesting agent is not the lowest-loaded
// among all online agents with matching capabilities, returns nil.
func (s *StoreScheduler) AssignTask(agentID string, capabilities []string) (*model.Task, error) {
	// Verify agent status
	agent, err := s.store.GetAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}
	if agent.Status != "online" {
		return nil, nil // non-online agents do not receive tasks
	}

	// Load balancing: check if requesting agent is the lowest-loaded candidate
	if !s.isLowestLoaded(agentID, capabilities) {
		return nil, nil
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

	// Persist assigned_to to database
	if err := s.store.UpdateTaskAssignment(task.TaskID, agentID); err != nil {
		// Log error but still return task (in-memory assigned_to is correct,
		// next heartbeat check can fix persistence)
		_ = err
	}

	task.Status = "assigned"
	task.AssignedTo = agentID
	return task, nil
}

// isLowestLoaded checks if the requesting agent has the lowest (or tied for lowest)
// active task count among all online agents with overlapping capabilities.
// Falls back to allowing assignment if any error occurs (degradation strategy).
func (s *StoreScheduler) isLowestLoaded(agentID string, capabilities []string) bool {
	agents, err := s.store.ListAgents()
	if err != nil {
		return true // degradation: allow assignment on error
	}

	// Filter online agents with at least one overlapping capability
	type candidate struct {
		id   string
		load int
	}
	var candidates []candidate
	for _, a := range agents {
		if a.Status != "online" {
			continue
		}
		if !hasOverlap(a.Capabilities, capabilities) {
			continue
		}
		count, err := s.store.CountAgentActiveTasks(a.AgentID)
		if err != nil {
			return true // degradation: allow assignment on error
		}
		candidates = append(candidates, candidate{id: a.AgentID, load: count})
	}

	// If only one candidate (or none found matching), allow assignment
	if len(candidates) <= 1 {
		return true
	}

	// Find minimum load
	minLoad := candidates[0].load
	for _, c := range candidates[1:] {
		if c.load < minLoad {
			minLoad = c.load
		}
	}

	// Check if requesting agent's load is at most the minimum
	for _, c := range candidates {
		if c.id == agentID {
			return c.load <= minLoad
		}
	}

	// Agent not found in candidates (shouldn't happen), allow assignment
	return true
}

// hasOverlap returns true if the two slices share at least one common element.
func hasOverlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}

// RequeueTask requeues a task (status back to pending) and clears assigned_to.
func (s *StoreScheduler) RequeueTask(taskID string) error {
	if err := s.queue.UpdateStatus(taskID, "pending", nil, ""); err != nil {
		return err
	}
	// Clear assigned_to when requeuing
	return s.store.UpdateTaskAssignment(taskID, "")
}
