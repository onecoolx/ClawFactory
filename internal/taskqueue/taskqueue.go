// Package taskqueue 实现任务队列，管理待执行任务的持久化队列
package taskqueue

import "github.com/clawfactory/clawfactory/internal/model"

// TaskQueue 任务队列接口
type TaskQueue interface {
	Enqueue(task model.Task) error
	Dequeue(capabilities []string) (*model.Task, error)
	UpdateStatus(taskID string, status string, output map[string]string, errMsg string) error
	GetTask(taskID string) (*model.Task, error)
	GetTasksByWorkflow(workflowID string) ([]model.Task, error)
	RestoreUnfinished() ([]model.Task, error)
}
