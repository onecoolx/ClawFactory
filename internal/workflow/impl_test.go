package workflow

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"pgregory.net/rapid"
)

func newTestWorkflowEngine(t testing.TB) (*StoreWorkflowEngine, *store.SQLiteStore) {
	tmpDB, err := os.CreateTemp("", "clawfactory-wf-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpDB.Close()
	t.Cleanup(func() { os.Remove(tmpDB.Name()) })

	s, err := store.NewSQLiteStore(tmpDB.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	q := taskqueue.NewStoreBackedQueue(s)
	return NewStoreWorkflowEngine(s, q), s
}

// Property 20: DAG 验证正确性
// **Validates: Requirements 14.2**
func TestProperty20_DAGValidation(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		e, _ := newTestWorkflowEngine(t)

		// 生成随机 DAG 或带环图
		hasCycle := rapid.Bool().Draw(t, "hasCycle")
		var def model.WorkflowDefinition
		if hasCycle {
			def = model.WorkflowDefinition{
				ID: "cycle", Name: "cycle",
				Nodes: []model.WorkflowNode{
					{ID: "a", Type: "t", Capabilities: []string{"c"}},
					{ID: "b", Type: "t", Capabilities: []string{"c"}},
					{ID: "c", Type: "t", Capabilities: []string{"c"}},
				},
				Edges: []model.WorkflowEdge{
					{From: "a", To: "b"}, {From: "b", To: "c"}, {From: "c", To: "a"},
				},
			}
		} else {
			n := rapid.IntRange(2, 5).Draw(t, "nodeCount")
			nodes := make([]model.WorkflowNode, n)
			ids := make([]string, n)
			for i := 0; i < n; i++ {
				ids[i] = rapid.StringMatching(`^n[a-z]$`).Draw(t, "nodeID")
				nodes[i] = model.WorkflowNode{ID: ids[i], Type: "t", Capabilities: []string{"c"}}
			}
			// 只添加前向边（保证无环）
			var edges []model.WorkflowEdge
			for i := 0; i < n-1; i++ {
				edges = append(edges, model.WorkflowEdge{From: ids[i], To: ids[i+1]})
			}
			def = model.WorkflowDefinition{ID: "dag", Name: "dag", Nodes: nodes, Edges: edges}
		}

		err := e.ValidateDAG(def)
		if hasCycle && err == nil {
			t.Fatal("expected error for cyclic graph")
		}
		if !hasCycle && err != nil {
			t.Fatalf("unexpected error for DAG: %v", err)
		}
	})
}

// Property 21: 工作流起始任务调度
// **Validates: Requirements 14.3**
func TestProperty21_WorkflowRootTaskScheduling(t *testing.T) {
	e, s := newTestWorkflowEngine(t)

	def := model.WorkflowDefinition{
		ID: "root-test", Name: "root",
		Nodes: []model.WorkflowNode{
			{ID: "a", Type: "t", Capabilities: []string{"c"}},
			{ID: "b", Type: "t", Capabilities: []string{"c"}},
			{ID: "c", Type: "t", Capabilities: []string{"c"}},
		},
		Edges: []model.WorkflowEdge{
			{From: "a", To: "c"}, {From: "b", To: "c"},
		},
	}

	inst, err := e.SubmitWorkflow(def)
	if err != nil {
		t.Fatal(err)
	}

	tasks, err := s.GetTasksByWorkflow(inst.InstanceID)
	if err != nil {
		t.Fatal(err)
	}

	// 起始节点应为 a 和 b（入度为 0）
	if len(tasks) != 2 {
		t.Fatalf("expected 2 root tasks, got %d", len(tasks))
	}
	nodeIDs := map[string]bool{}
	for _, tk := range tasks {
		nodeIDs[tk.NodeID] = true
	}
	if !nodeIDs["a"] || !nodeIDs["b"] {
		t.Fatalf("expected root nodes a and b, got %v", nodeIDs)
	}
}

// Property 22: 下游任务依赖满足后调度
// **Validates: Requirements 14.4**
func TestProperty22_DownstreamSchedulingOnDependencySatisfied(t *testing.T) {
	e, s := newTestWorkflowEngine(t)

	def := model.WorkflowDefinition{
		ID: "downstream-test", Name: "ds",
		Nodes: []model.WorkflowNode{
			{ID: "a", Type: "t", Capabilities: []string{"c"}},
			{ID: "b", Type: "t", Capabilities: []string{"c"}},
		},
		Edges: []model.WorkflowEdge{{From: "a", To: "b"}},
	}

	inst, err := e.SubmitWorkflow(def)
	if err != nil {
		t.Fatal(err)
	}

	// 只有 a 应该被入队
	tasks, _ := s.GetTasksByWorkflow(inst.InstanceID)
	if len(tasks) != 1 || tasks[0].NodeID != "a" {
		t.Fatalf("expected only task a, got %v", tasks)
	}

	// 完成 task a
	taskAID := tasks[0].TaskID
	s.UpdateTaskStatus(taskAID, "completed", map[string]string{"out": "done"}, "")
	if err := e.OnTaskCompleted(taskAID); err != nil {
		t.Fatal(err)
	}

	// 现在 b 应该被入队
	tasks, _ = s.GetTasksByWorkflow(inst.InstanceID)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after completing a, got %d", len(tasks))
	}
}

// Property 23: 工作流状态推导
// **Validates: Requirements 14.5, 14.6**
func TestProperty23_WorkflowStatusDerivation(t *testing.T) {
	e, s := newTestWorkflowEngine(t)

	// 测试失败场景
	def := model.WorkflowDefinition{
		ID: "status-test", Name: "st",
		Nodes: []model.WorkflowNode{
			{ID: "a", Type: "t", Capabilities: []string{"c"}},
		},
		Edges: []model.WorkflowEdge{},
	}

	inst, err := e.SubmitWorkflow(def)
	if err != nil {
		t.Fatal(err)
	}

	tasks, _ := s.GetTasksByWorkflow(inst.InstanceID)
	taskAID := tasks[0].TaskID

	// 永久失败
	s.UpdateTaskStatus(taskAID, "failed", nil, "permanent error")
	if err := e.OnTaskPermanentlyFailed(taskAID); err != nil {
		t.Fatal(err)
	}

	wf, err := e.GetWorkflowStatus(inst.InstanceID)
	if err != nil {
		t.Fatal(err)
	}
	if wf.Status != "failed" {
		t.Fatalf("workflow should be failed, got %s", wf.Status)
	}
}

// Property 24: 工作流定义序列化往返
// **Validates: Requirements 14.8**
func TestProperty24_WorkflowDefinitionSerializationRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(t, "nodeCount")
		nodes := make([]model.WorkflowNode, n)
		for i := 0; i < n; i++ {
			nodes[i] = model.WorkflowNode{
				ID:           rapid.StringMatching(`^n[a-z]{2}$`).Draw(t, "nodeID"),
				Type:         rapid.StringMatching(`^[a-z]{3,6}$`).Draw(t, "type"),
				Capabilities: []string{rapid.StringMatching(`^[a-z]{3}$`).Draw(t, "cap")},
				Priority:     rapid.IntRange(0, 10).Draw(t, "prio"),
			}
		}
		var edges []model.WorkflowEdge
		for i := 0; i < n-1; i++ {
			edges = append(edges, model.WorkflowEdge{From: nodes[i].ID, To: nodes[i+1].ID})
		}

		def := model.WorkflowDefinition{
			ID:    rapid.StringMatching(`^wf-[a-z]{3}$`).Draw(t, "defID"),
			Name:  rapid.StringMatching(`^[a-z]{3,8}$`).Draw(t, "name"),
			Nodes: nodes,
			Edges: edges,
		}

		data, err := json.Marshal(def)
		if err != nil {
			t.Fatal(err)
		}
		var got model.WorkflowDefinition
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.ID != def.ID || got.Name != def.Name {
			t.Fatalf("round-trip mismatch: %+v vs %+v", def, got)
		}
		if len(got.Nodes) != len(def.Nodes) || len(got.Edges) != len(def.Edges) {
			t.Fatalf("round-trip count mismatch")
		}
	})
}

// Unit test: complete workflow marks as completed
func TestWorkflowCompletedOnAllTasksDone(t *testing.T) {
	e, s := newTestWorkflowEngine(t)

	def := model.WorkflowDefinition{
		ID: "complete-test", Name: "ct",
		Nodes: []model.WorkflowNode{
			{ID: "only", Type: "t", Capabilities: []string{"c"}},
		},
		Edges: []model.WorkflowEdge{},
	}

	inst, err := e.SubmitWorkflow(def)
	if err != nil {
		t.Fatal(err)
	}

	tasks, _ := s.GetTasksByWorkflow(inst.InstanceID)
	s.UpdateTaskStatus(tasks[0].TaskID, "completed", nil, "")
	e.OnTaskCompleted(tasks[0].TaskID)

	wf, _ := e.GetWorkflowStatus(inst.InstanceID)
	if wf.Status != "completed" {
		t.Fatalf("workflow should be completed, got %s", wf.Status)
	}
}

// Ensure unused import is used
var _ = time.Now
