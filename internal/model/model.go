// Package model 定义平台核心数据模型和请求/响应结构体
package model

import "time"

// --- 核心数据模型 ---

// AgentInfo 智能体信息
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

// Task 任务
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

// WorkflowDefinition 工作流定义（DAG）
type WorkflowDefinition struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Nodes []WorkflowNode `json:"nodes"`
	Edges []WorkflowEdge `json:"edges"`
}

// WorkflowNode 工作流节点
type WorkflowNode struct {
	ID           string            `json:"id"`
	Type         string            `json:"type"`
	Capabilities []string          `json:"capabilities"`
	Input        map[string]string `json:"input,omitempty"`
	Priority     int               `json:"priority,omitempty"`
}

// WorkflowEdge 工作流边（依赖关系）
type WorkflowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// WorkflowInstance 工作流实例
type WorkflowInstance struct {
	InstanceID   string    `json:"instance_id"`
	DefinitionID string    `json:"definition_id"`
	Status       string    `json:"status"` // "running", "completed", "failed"
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Artifact 产出物
type Artifact struct {
	WorkflowID string    `json:"workflow_id"`
	TaskID     string    `json:"task_id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	CreatedAt  time.Time `json:"created_at"`
}

// LogEntry 日志条目
type LogEntry struct {
	AgentID   string `json:"agent_id"`
	TaskID    string `json:"task_id,omitempty"`
	Level     string `json:"level"` // "info", "warn", "error"
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// AuditLogEntry 审计日志条目
type AuditLogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	AgentID   string    `json:"agent_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Allowed   bool      `json:"allowed"`
	Reason    string    `json:"reason"`
}

// --- ARI 请求/响应结构体 ---

// RegisterRequest 注册请求
type RegisterRequest struct {
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}

// RegisterResponse 注册响应
type RegisterResponse struct {
	AgentID string `json:"agent_id"`
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	AgentID string `json:"agent_id"`
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Status string `json:"status"`
}

// TaskResponse 任务拉取响应
type TaskResponse struct {
	TaskID       string            `json:"task_id,omitempty"`
	WorkflowID   string            `json:"workflow_id,omitempty"`
	Type         string            `json:"type,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Input        map[string]string `json:"input,omitempty"`
	Assigned     bool              `json:"assigned"`
}

// TaskStatusUpdate 任务状态更新请求
type TaskStatusUpdate struct {
	AgentID string            `json:"agent_id"`
	Status  string            `json:"status"` // "running", "completed", "failed"
	Output  map[string]string `json:"output,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// AuthorizeRequest 授权请求
type AuthorizeRequest struct {
	AgentID  string `json:"agent_id"`
	Action   string `json:"action"`   // "call_tool", "read_memory", "write_memory"
	Resource string `json:"resource"` // 工具名或资源路径
}

// AuthorizeResponse 授权响应
type AuthorizeResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// ErrorResponse 统一错误响应
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail 错误详情
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// --- 策略配置结构体 ---

// PolicyConfig 策略配置（从 JSON 文件加载）
type PolicyConfig struct {
	MaxRetries                 int                        `json:"max_retries"`
	HeartbeatIntervalSeconds   int                        `json:"heartbeat_interval_seconds"`
	HeartbeatTimeoutMultiplier int                        `json:"heartbeat_timeout_multiplier"`
	Roles                      map[string]RoleDefinition  `json:"roles"`
	ToolWhitelist              map[string]ToolPolicy      `json:"tool_whitelist"`
}

// RoleDefinition 角色定义
type RoleDefinition struct {
	Permissions []Permission `json:"permissions"`
}

// Permission 权限
type Permission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}

// ToolPolicy 工具策略
type ToolPolicy struct {
	AllowedTools []string `json:"allowed_tools"`
	RateLimit    int      `json:"rate_limit_per_minute"`
}
