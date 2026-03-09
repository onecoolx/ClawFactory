// Package model defines core data models and request/response structs for the platform.
package model

import "time"

// --- Core Data Models ---

// AgentInfo represents a registered agent.
type AgentInfo struct {
	AgentID       string    `json:"agent_id"`
	Name          string    `json:"name"`
	Capabilities  []string  `json:"capabilities"`
	Version       string    `json:"version"`
	Status        string    `json:"status"` // "online", "offline", "deregistered"
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Roles         []string  `json:"roles"`
	RegisteredAt  time.Time `json:"registered_at"`
}

// Task represents a unit of work in a workflow.
type Task struct {
	TaskID       string            `json:"task_id"`
	WorkflowID   string            `json:"workflow_id"`
	NodeID       string            `json:"node_id"`
	Type         string            `json:"type"`
	Capabilities []string          `json:"capabilities"`
	Input        map[string]string `json:"input"`
	Output       map[string]string `json:"output"`
	Status       string            `json:"status"` // "pending", "assigned", "running", "completed", "failed"
	Priority     int               `json:"priority"`
	AssignedTo   string            `json:"assigned_to"`
	RetryCount   int               `json:"retry_count"`
	Error        string            `json:"error"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// WorkflowDefinition defines a DAG workflow.
type WorkflowDefinition struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Nodes []WorkflowNode `json:"nodes"`
	Edges []WorkflowEdge `json:"edges"`
}

// WorkflowNode represents a node in the workflow DAG.
type WorkflowNode struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Capabilities []string          `json:"capabilities"`
	Input        map[string]string `json:"input,omitempty"`
	Priority     int               `json:"priority,omitempty"`
}

// WorkflowEdge represents a dependency edge in the DAG.
type WorkflowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// WorkflowInstance represents a running instance of a workflow.
type WorkflowInstance struct {
	InstanceID   string    `json:"instance_id"`
	DefinitionID string    `json:"definition_id"`
	Status       string    `json:"status"` // "running", "completed", "failed"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Artifact represents a workflow output artifact metadata entry.
type Artifact struct {
	WorkflowID string    `json:"workflow_id"`
	TaskID     string    `json:"task_id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	CreatedAt  time.Time `json:"created_at"`
}

// LogEntry represents an agent log entry.
type LogEntry struct {
	AgentID   string `json:"agent_id"`
	TaskID    string `json:"task_id"`
	Level     string `json:"level"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// AuditLogEntry represents a security audit log entry.
type AuditLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	AgentID   string    `json:"agent_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Allowed   bool      `json:"allowed"`
	Reason    string    `json:"reason"`
}

// --- ARI Protocol Request/Response Structs ---

// RegisterRequest is the request body for agent registration.
type RegisterRequest struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}

// RegisterResponse is the response body for agent registration.
type RegisterResponse struct {
	AgentID string `json:"agent_id"`
}

// HeartbeatRequest is the request body for agent heartbeat.
type HeartbeatRequest struct {
	AgentID string `json:"agent_id"`
}

// HeartbeatResponse is the response body for agent heartbeat.
type HeartbeatResponse struct {
	Status string `json:"status"`
}

// TaskResponse is the response body for task pull.
type TaskResponse struct {
	TaskID       string            `json:"task_id,omitempty"`
	WorkflowID   string            `json:"workflow_id,omitempty"`
	Type         string            `json:"type,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Input        map[string]string `json:"input,omitempty"`
	Assigned     bool              `json:"assigned"`
}

// TaskStatusUpdate is the request body for updating task status.
type TaskStatusUpdate struct {
	Status string            `json:"status"`
	Output map[string]string `json:"output,omitempty"`
	Error  string            `json:"error,omitempty"`
}

// AuthorizeRequest is the request body for authorization check.
type AuthorizeRequest struct {
	AgentID  string `json:"agent_id"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

// AuthorizeResponse is the response body for authorization check.
type AuthorizeResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// ErrorResponse is the standard error response.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error code and message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- Policy Configuration Structs ---

// PolicyConfig holds the complete policy configuration.
type PolicyConfig struct {
	MaxRetries    int                       `json:"max_retries"`
	Roles         map[string]RoleDefinition `json:"roles"`
	ToolWhitelist map[string]ToolPolicy     `json:"tool_whitelist"`
}

// RoleDefinition defines permissions for a role.
type RoleDefinition struct {
	Permissions []Permission `json:"permissions"`
}

// Permission defines an access control rule.
type Permission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}

// ToolPolicy defines allowed tools and rate limits for a role.
type ToolPolicy struct {
	AllowedTools []string `json:"allowed_tools"`
	RateLimit    int      `json:"rate_limit"`
}
