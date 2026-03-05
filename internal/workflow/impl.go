package workflow

import (
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
)

// StoreWorkflowEngine 基于 StateStore 的工作流引擎实现
type StoreWorkflowEngine struct {
	store store.StateStore
	queue taskqueue.TaskQueue
}

// NewStoreWorkflowEngine 创建工作流引擎
func NewStoreWorkflowEngine(s store.StateStore, q taskqueue.TaskQueue) *StoreWorkflowEngine {
	return &StoreWorkflowEngine{store: s, queue: q}
}

// ValidateDAG 验证工作流定义是否为合法 DAG（无环检测）
func (e *StoreWorkflowEngine) ValidateDAG(def model.WorkflowDefinition) error {
	// 构建邻接表和入度表
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	nodeSet := make(map[string]bool)

	for _, n := range def.Nodes {
		nodeSet[n.ID] = true
		inDegree[n.ID] = 0
	}
	for _, edge := range def.Edges {
		if !nodeSet[edge.From] || !nodeSet[edge.To] {
			return fmt.Errorf("edge references unknown node: %s -> %s", edge.From, edge.To)
		}
		adj[edge.From] = append(adj[edge.From], edge.To)
		inDegree[edge.To]++
	}

	// 拓扑排序（Kahn's algorithm）
	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		visited++
		for _, next := range adj[node] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if visited != len(nodeSet) {
		return fmt.Errorf("workflow contains a cycle")
	}
	return nil
}

// SubmitWorkflow 提交工作流：验证 → 持久化 → 调度起始任务
func (e *StoreWorkflowEngine) SubmitWorkflow(def model.WorkflowDefinition) (model.WorkflowInstance, error) {
	if err := e.ValidateDAG(def); err != nil {
		return model.WorkflowInstance{}, fmt.Errorf("invalid DAG: %w", err)
	}

	now := time.Now()
	instanceID := fmt.Sprintf("wfi-%s-%d", def.ID, now.UnixNano())
	instance := model.WorkflowInstance{
		InstanceID:   instanceID,
		DefinitionID: def.ID,
		Status:       "running",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := e.store.SaveWorkflow(instance, def); err != nil {
		return model.WorkflowInstance{}, fmt.Errorf("save workflow: %w", err)
	}

	// 找到起始节点（入度为 0）并入队
	inDegree := make(map[string]int)
	for _, n := range def.Nodes {
		inDegree[n.ID] = 0
	}
	for _, edge := range def.Edges {
		inDegree[edge.To]++
	}

	for _, node := range def.Nodes {
		if inDegree[node.ID] == 0 {
			task := model.Task{
				TaskID:       fmt.Sprintf("%s-%s", instanceID, node.ID),
				WorkflowID:   instanceID,
				NodeID:       node.ID,
				Type:         node.Type,
				Capabilities: node.Capabilities,
				Input:        node.Input,
				Output:       map[string]string{},
				Priority:     node.Priority,
			}
			if err := e.queue.Enqueue(task); err != nil {
				return model.WorkflowInstance{}, fmt.Errorf("enqueue task: %w", err)
			}
		}
	}

	return instance, nil
}

// OnTaskCompleted 任务完成回调
func (e *StoreWorkflowEngine) OnTaskCompleted(taskID string) error {
	task, err := e.store.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}

	inst, def, err := e.store.GetWorkflow(task.WorkflowID)
	if err != nil {
		return fmt.Errorf("get workflow: %w", err)
	}
	if inst.Status != "running" {
		return nil
	}

	// 找到下游节点
	downstream := make(map[string]bool)
	for _, edge := range def.Edges {
		if edge.From == task.NodeID {
			downstream[edge.To] = true
		}
	}

	// 检查下游节点的所有依赖是否满足
	tasks, err := e.store.GetTasksByWorkflow(task.WorkflowID)
	if err != nil {
		return err
	}
	taskStatus := make(map[string]string)
	for _, t := range tasks {
		taskStatus[t.NodeID] = t.Status
	}

	for nodeID := range downstream {
		allDepsCompleted := true
		for _, edge := range def.Edges {
			if edge.To == nodeID {
				if taskStatus[edge.From] != "completed" {
					allDepsCompleted = false
					break
				}
			}
		}
		if allDepsCompleted {
			// 找到节点定义
			for _, node := range def.Nodes {
				if node.ID == nodeID {
					newTask := model.Task{
						TaskID:       fmt.Sprintf("%s-%s", task.WorkflowID, node.ID),
						WorkflowID:   task.WorkflowID,
						NodeID:       node.ID,
						Type:         node.Type,
						Capabilities: node.Capabilities,
						Input:        node.Input,
						Output:       map[string]string{},
						Priority:     node.Priority,
					}
					if err := e.queue.Enqueue(newTask); err != nil {
						return fmt.Errorf("enqueue downstream: %w", err)
					}
					break
				}
			}
		}
	}

	// 检查是否所有任务完成
	allCompleted := true
	for _, t := range tasks {
		if t.TaskID == taskID {
			continue // 当前任务已完成
		}
		if t.Status != "completed" {
			allCompleted = false
			break
		}
	}
	// 还需要检查是否所有节点都有对应任务
	if allCompleted && len(tasks) >= len(def.Nodes) {
		e.store.UpdateWorkflowStatus(task.WorkflowID, "completed")
	}

	return nil
}

// OnTaskPermanentlyFailed 任务永久失败回调
func (e *StoreWorkflowEngine) OnTaskPermanentlyFailed(taskID string) error {
	task, err := e.store.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	return e.store.UpdateWorkflowStatus(task.WorkflowID, "failed")
}

// GetWorkflowStatus 获取工作流状态
func (e *StoreWorkflowEngine) GetWorkflowStatus(instanceID string) (model.WorkflowInstance, error) {
	inst, _, err := e.store.GetWorkflow(instanceID)
	return inst, err
}
