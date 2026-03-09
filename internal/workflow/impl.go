package workflow

import (
	"fmt"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
)

// StoreWorkflowEngine is the StateStore-based workflow engine implementation.
type StoreWorkflowEngine struct {
	store store.StateStore
	queue taskqueue.TaskQueue
}

// NewStoreWorkflowEngine creates a new workflow engine.
func NewStoreWorkflowEngine(s store.StateStore, q taskqueue.TaskQueue) *StoreWorkflowEngine {
	return &StoreWorkflowEngine{store: s, queue: q}
}

// ValidateDAG validates that the workflow definition is a valid DAG (cycle detection).
func (e *StoreWorkflowEngine) ValidateDAG(def model.WorkflowDefinition) error {
	// Build adjacency list and in-degree map
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

	// Topological sort (Kahn's algorithm)
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

// SubmitWorkflow submits a workflow: validate -> persist -> schedule root tasks.
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

	// Find root nodes (in-degree 0) and enqueue them
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

// OnTaskCompleted is the task completion callback.
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

	// Find downstream nodes
	downstream := make(map[string]bool)
	for _, edge := range def.Edges {
		if edge.From == task.NodeID {
			downstream[edge.To] = true
		}
	}

	// Check if all dependencies of downstream nodes are satisfied
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
			// Find node definition
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

	// Check if all tasks are completed
	allCompleted := true
	for _, t := range tasks {
		if t.TaskID == taskID {
			continue // current task is already completed
		}
		if t.Status != "completed" {
			allCompleted = false
			break
		}
	}
	// Also check if all nodes have corresponding tasks
	if allCompleted && len(tasks) >= len(def.Nodes) {
		e.store.UpdateWorkflowStatus(task.WorkflowID, "completed")
	}

	return nil
}

// OnTaskPermanentlyFailed is the permanent task failure callback.
func (e *StoreWorkflowEngine) OnTaskPermanentlyFailed(taskID string) error {
	task, err := e.store.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	return e.store.UpdateWorkflowStatus(task.WorkflowID, "failed")
}

// GetWorkflowStatus returns the workflow status.
func (e *StoreWorkflowEngine) GetWorkflowStatus(instanceID string) (model.WorkflowInstance, error) {
	inst, _, err := e.store.GetWorkflow(instanceID)
	return inst, err
}
