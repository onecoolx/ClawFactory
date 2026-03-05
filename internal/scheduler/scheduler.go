// Package scheduler 实现调度器，负责将任务分配给合适的智能体
package scheduler

import "github.com/clawfactory/clawfactory/internal/model"

// Scheduler 调度器接口
type Scheduler interface {
	AssignTask(agentID string, capabilities []string) (*model.Task, error)
	RequeueTask(taskID string) error
}
