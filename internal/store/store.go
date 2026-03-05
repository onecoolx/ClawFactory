// Package store 定义状态存储接口，当前实现为 SQLite
package store

import (
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
)

// StateStore 状态存储接口
type StateStore interface {
	// 智能体状态
	SaveAgent(agent model.AgentInfo) error
	GetAgent(agentID string) (model.AgentInfo, error)
	ListAgents() ([]model.AgentInfo, error)
	UpdateAgentStatus(agentID string, status string, lastHeartbeat time.Time) error

	// 任务状态
	SaveTask(task model.Task) error
	GetTask(taskID string) (model.Task, error)
	GetTasksByWorkflow(workflowID string) ([]model.Task, error)
	UpdateTaskStatus(taskID string, status string, output map[string]string, errMsg string) error

	// 工作流状态
	SaveWorkflow(instance model.WorkflowInstance, definition model.WorkflowDefinition) error
	GetWorkflow(instanceID string) (model.WorkflowInstance, model.WorkflowDefinition, error)
	UpdateWorkflowStatus(instanceID string, status string) error

	// 日志
	SaveLog(entry model.LogEntry) error
	GetLogs(agentID string, taskID string, since time.Time, until time.Time) ([]model.LogEntry, error)

	// 产出物元数据
	SaveArtifact(artifact model.Artifact) error
	GetArtifacts(workflowID string) ([]model.Artifact, error)

	// 审计日志
	SaveAuditLog(entry model.AuditLogEntry) error
}
