// Package scheduler implements the task scheduler for assigning tasks to suitable agents.
package scheduler

import "github.com/clawfactory/clawfactory/internal/model"

// Scheduler is the task scheduler interface.
type Scheduler interface {
	AssignTask(agentID string, capabilities []string) (*model.Task, error)
	RequeueTask(taskID string) error
}
