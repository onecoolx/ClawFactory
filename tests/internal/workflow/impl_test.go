package workflow_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"github.com/clawfactory/clawfactory/internal/taskqueue"
	"github.com/clawfactory/clawfactory/internal/workflow"
	"pgregory.net/rapid"
)

func newTestWorkflowEngine(t *testing.T) (*workflow.StoreWorkflowEngine, *store.SQLiteStore) {
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
	return workflow.NewStoreWorkflowEngine(s, q), s
}

// Property 20: DAG validation correctness
// **Validates: Requirements 14.2**
func TestProperty20_DAGValidation(t *testing.T) {
	e, _ := newTestWorkflowEngine(t)

	rapid.Check(t, func(rt *rapid.T) {
		hasCycle := rapid.Bool().Draw(rt, "hasCycle")
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
			n := rapid.IntRange(2, 5).Draw(rt, "nodeCount")
			nodes := make([]model.WorkflowNode, n)
			ids := make([]string, n)
			// Use index to ensure unique node IDs
			for i := 0; i < n; i++ {
				ids[i] = fmt.Sprintf("n%d", i)
				nodes[i] = model.WorkflowNode{ID: ids[i], Type: "t", Capabilities: []string{"c"}}
			}
			// Only add forward edges (guarantees acyclic)
			var edges []model.WorkflowEdge
			for i := 0; i < n-1; i++ {
				edges = append(edges, model.WorkflowEdge{From: ids[i], To: ids[i+1]})
			}
			def = model.WorkflowDefinition{ID: "dag", Name: "dag", Nodes: nodes, Edges: edges}
		}

		err := e.ValidateDAG(def)
		if hasCycle && err == nil {
			rt.Fatal("expected error for cyclic graph")
		}
		if !hasCycle && err != nil {
			rt.Fatalf("unexpected error for DAG: %v", err)
		}
	})
}

// Property 21: Workflow root task scheduling
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

// Property 22: Downstream task scheduling after dependency satisfaction
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

	tasks, _ := s.GetTasksByWorkflow(inst.InstanceID)
	if len(tasks) != 1 || tasks[0].NodeID != "a" {
		t.Fatalf("expected only task a, got %v", tasks)
	}

	taskAID := tasks[0].TaskID
	s.UpdateTaskStatus(taskAID, "completed", map[string]string{"out": "done"}, "")
	if err := e.OnTaskCompleted(taskAID); err != nil {
		t.Fatal(err)
	}

	tasks, _ = s.GetTasksByWorkflow(inst.InstanceID)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks after completing a, got %d", len(tasks))
	}
}

// Property 23: Workflow status derivation
// **Validates: Requirements 14.5, 14.6**
func TestProperty23_WorkflowStatusDerivation(t *testing.T) {
	e, s := newTestWorkflowEngine(t)

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

// Property 24: Workflow definition serialization round-trip
// **Validates: Requirements 14.8**
func TestProperty24_WorkflowDefinitionSerializationRoundTrip(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 4).Draw(rt, "nodeCount")
		nodes := make([]model.WorkflowNode, n)
		for i := 0; i < n; i++ {
			nodes[i] = model.WorkflowNode{
				ID:           "n" + rapid.StringMatching("[a-z]{2}").Draw(rt, "nodeID"),
				Type:         rapid.StringMatching("[a-z]{3,6}").Draw(rt, "type"),
				Capabilities: []string{rapid.StringMatching("[a-z]{3}").Draw(rt, "cap")},
				Priority:     rapid.IntRange(0, 10).Draw(rt, "prio"),
			}
		}
		var edges []model.WorkflowEdge
		for i := 0; i < n-1; i++ {
			edges = append(edges, model.WorkflowEdge{From: nodes[i].ID, To: nodes[i+1].ID})
		}

		def := model.WorkflowDefinition{
			ID:    "wf-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "defID"),
			Name:  rapid.StringMatching("[a-z]{3,8}").Draw(rt, "name"),
			Nodes: nodes,
			Edges: edges,
		}

		data, err := json.Marshal(def)
		if err != nil {
			rt.Fatal(err)
		}
		var got model.WorkflowDefinition
		if err := json.Unmarshal(data, &got); err != nil {
			rt.Fatal(err)
		}
		if got.ID != def.ID || got.Name != def.Name {
			rt.Fatalf("round-trip mismatch: %+v vs %+v", def, got)
		}
		if len(got.Nodes) != len(def.Nodes) || len(got.Edges) != len(def.Edges) {
			rt.Fatalf("round-trip count mismatch")
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
