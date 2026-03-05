// Package store defines the state store interface, currently implemented with SQLite.
package store

import (
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
)

// StateStore is the state storage interface.
type StateStore interface {
	// Agent state
	SaveAgent(agent model.AgentInfo) error
	GetAgent(agentID string) (model.AgentInfo, error)
	ListAgents() ([]model.AgentInfo, error)
	UpdateAgentStatus(agentID string, status string, lastHeartbeat time.Time) error

	// Task state
	SaveTask(task model.Task) error
	GetTask(taskID string) (model.Task, error)
	GetTasksByWorkflow(workflowID string) ([]model.Task, error)
	UpdateTaskStatus(taskID string, status string, output map[string]string, errMsg string) error

	// Workflow state
	SaveWorkflow(instance model.WorkflowInstance, definition model.WorkflowDefinition) error
	GetWorkflow(instanceID string) (model.WorkflowInstance, model.WorkflowDefinition, error)
	UpdateWorkflowStatus(instanceID string, status string) error

	// Logs
	SaveLog(entry model.LogEntry) error
	GetLogs(agentID string, taskID string, since time.Time, until time.Time) ([]model.LogEntry, error)

	// Artifact metadata
	SaveArtifact(artifact model.Artifact) error
	GetArtifacts(workflowID string) ([]model.Artifact, error)

	// Audit logs
	SaveAuditLog(entry model.AuditLogEntry) error
}
