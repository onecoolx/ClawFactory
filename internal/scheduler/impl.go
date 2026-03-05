package scheduler

import (
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
)

// StoreScheduler 基于 StateStore 的调度器实现
type StoreScheduler struct {
	store store.StateStore
	queue taskqueue.TaskQueue
}

// NewStoreScheduler 创建调度器
func NewStoreScheduler(s store.StateStore, q taskqueue.TaskQueue) *StoreScheduler {
	return &StoreScheduler{store: s, queue: q}
}

// AssignTask 为指定智能体分配匹配的任务
func (s *StoreScheduler) AssignTask(agentID string, capabilities []string) (*model.Task, error) {
	// 验证智能体状态
	agent, err := s.store.GetAgent(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}
	if agent.Status != "online" {
		return nil, nil // 非 online 智能体不分配任务
	}

	// 从队列中按能力匹配出队
	task, err := s.queue.Dequeue(capabilities)
	if err != nil {
		return nil, fmt.Errorf("dequeue: %w", err)
	}
	if task == nil {
		return nil, nil // 无匹配任务
	}

	// 更新任务状态为 assigned
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

// RequeueTask 将任务重新入队（状态回到 pending）
func (s *StoreScheduler) RequeueTask(taskID string) error {
	return s.queue.UpdateStatus(taskID, "pending", nil, "")
}
