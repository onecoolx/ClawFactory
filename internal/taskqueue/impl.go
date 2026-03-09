package taskqueue

import (
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

// Dequeue queries pending tasks via StateStore interface, returns the first task with highest priority and matching capabilities.
func (q *StoreBackedQueue) Dequeue(capabilities []string) (*model.Task, error) {
	tasks, err := q.store.ListPendingTasks(capabilities)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	return &tasks[0], nil
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

// RestoreUnfinished queries all unfinished tasks (pending/assigned/running) via StateStore interface.
func (q *StoreBackedQueue) RestoreUnfinished() ([]model.Task, error) {
	return q.store.ListUnfinishedTasks()
}
