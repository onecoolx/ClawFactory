// Package taskqueue implements the task queue for managing a persistent queue of pending tasks.
package taskqueue

import "github.com/clawfactory/clawfactory/internal/model"

// TaskQueue is the task queue interface.
type TaskQueue interface {
	Enqueue(task model.Task) error
	Dequeue(capabilities []string) (*model.Task, error)
	UpdateStatus(taskID string, status string, output map[string]string, errMsg string) error
	GetTask(taskID string) (*model.Task, error)
	GetTasksByWorkflow(workflowID string) ([]model.Task, error)
	RestoreUnfinished() ([]model.Task, error)
}
