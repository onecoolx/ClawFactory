// Package workflow implements the workflow engine for managing DAG workflow lifecycles.
package workflow

import "github.com/clawfactory/clawfactory/internal/model"

// WorkflowEngine is the workflow engine interface.
type WorkflowEngine interface {
	SubmitWorkflow(def model.WorkflowDefinition) (model.WorkflowInstance, error)
	ValidateDAG(def model.WorkflowDefinition) error
	OnTaskCompleted(taskID string) error
	OnTaskPermanentlyFailed(taskID string) error
	GetWorkflowStatus(instanceID string) (model.WorkflowInstance, error)
}
