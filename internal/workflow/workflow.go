// Package workflow 实现工作流引擎，管理 DAG 工作流的生命周期
package workflow

import "github.com/clawfactory/clawfactory/internal/model"

// WorkflowEngine 工作流引擎接口
type WorkflowEngine interface {
	SubmitWorkflow(def model.WorkflowDefinition) (model.WorkflowInstance, error)
	ValidateDAG(def model.WorkflowDefinition) error
	OnTaskCompleted(taskID string) error
	OnTaskPermanentlyFailed(taskID string) error
	GetWorkflowStatus(instanceID string) (model.WorkflowInstance, error)
}
