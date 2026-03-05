package taskqueue

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// StoreBackedQueue 基于 StateStore 的任务队列实现
type StoreBackedQueue struct {
	store store.StateStore
}

// NewStoreBackedQueue 创建任务队列
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

// Dequeue 按优先级和能力匹配出队
// 从所有 pending 任务中找到能力匹配且优先级最高的任务
func (q *StoreBackedQueue) Dequeue(capabilities []string) (*model.Task, error) {
	// 获取所有 pending 任务（通过遍历所有 workflow 的任务）
	// 这里简化实现：直接查询 store 底层
	// 由于 StateStore 接口不提供全局 pending 查询，我们需要通过底层实现
	// 为了保持接口一致性，我们在 StoreBackedQueue 中直接使用 SQLite
	sqlStore, ok := q.store.(*store.SQLiteStore)
	if !ok {
		return nil, fmt.Errorf("store backend must be SQLiteStore for Dequeue")
	}
	return q.dequeueFromSQLite(sqlStore, capabilities)
}

func (q *StoreBackedQueue) dequeueFromSQLite(s *store.SQLiteStore, capabilities []string) (*model.Task, error) {
	// 查询所有 pending 任务，按优先级降序
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
	return nil, nil // 无匹配任务
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
