package taskqueue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// StoreBackedQueue is the StateStore-based task queue implementation.
type StoreBackedQueue struct {
	store store.StateStore
}

// NewStoreBackedQueue creates a new task queue.
func NewStoreBackedQueue(s store.StateStore) *StoreBackedQueue {
	return &StoreBackedQueue{store: s}
}

func (q *StoreBackedQueue) Enqueue(task model.Task) error {
	task.Status = "pending"
	if task.CreatedAt.IsZero() {
		task.CreatedAt = time.Now()
	}
	task.UpdatedAt = time.Now()
	return q.store.SaveTask(task)
}

// Dequeue dequeues a task by priority and capability match.
// Finds the highest-priority pending task that matches the given capabilities.
func (q *StoreBackedQueue) Dequeue(capabilities []string) (*model.Task, error) {
	// Get all pending tasks (by querying the store backend).
	// Since the StateStore interface doesn't provide a global pending query,
	// we need to access the underlying implementation directly.
	// For interface consistency, we use SQLite directly in StoreBackedQueue.
	sqlStore, ok := q.store.(*store.SQLiteStore)
	if !ok {
		return nil, fmt.Errorf("store backend must be SQLiteStore for Dequeue")
	}
	return q.dequeueFromSQLite(sqlStore, capabilities)
}

func (q *StoreBackedQueue) dequeueFromSQLite(s *store.SQLiteStore, capabilities []string) (*model.Task, error) {
	// Query all pending tasks, ordered by priority descending
	rows, err := s.DB().Query(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE status = 'pending' ORDER BY priority DESC, created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var t model.Task
		var caps, input, output string
		if err := rows.Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
			&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(caps), &t.Capabilities)
		json.Unmarshal([]byte(input), &t.Input)
		json.Unmarshal([]byte(output), &t.Output)

		if matchCapabilities(t.Capabilities, capabilities) {
			return &t, nil
		}
	}
	return nil, nil // no matching task
}

func matchCapabilities(taskCaps, agentCaps []string) bool {
	capSet := make(map[string]bool)
	for _, c := range agentCaps {
		capSet[c] = true
	}
	for _, c := range taskCaps {
		if capSet[c] {
			return true
		}
	}
	return false
}

func (q *StoreBackedQueue) UpdateStatus(taskID string, status string, output map[string]string, errMsg string) error {
	return q.store.UpdateTaskStatus(taskID, status, output, errMsg)
}

func (q *StoreBackedQueue) GetTask(taskID string) (*model.Task, error) {
	t, err := q.store.GetTask(taskID)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (q *StoreBackedQueue) GetTasksByWorkflow(workflowID string) ([]model.Task, error) {
	return q.store.GetTasksByWorkflow(workflowID)
}

func (q *StoreBackedQueue) RestoreUnfinished() ([]model.Task, error) {
	sqlStore, ok := q.store.(*store.SQLiteStore)
	if !ok {
		return nil, fmt.Errorf("store backend must be SQLiteStore for RestoreUnfinished")
	}
	rows, err := sqlStore.DB().Query(
		`SELECT task_id, workflow_id, node_id, type, capabilities, input, output, status, priority, assigned_to, retry_count, error, created_at, updated_at
		 FROM tasks WHERE status IN ('pending', 'assigned', 'running') ORDER BY priority DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []model.Task
	for rows.Next() {
		var t model.Task
		var caps, input, output string
		if err := rows.Scan(&t.TaskID, &t.WorkflowID, &t.NodeID, &t.Type, &caps, &input, &output,
			&t.Status, &t.Priority, &t.AssignedTo, &t.RetryCount, &t.Error, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(caps), &t.Capabilities)
		json.Unmarshal([]byte(input), &t.Input)
		json.Unmarshal([]byte(output), &t.Output)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}
