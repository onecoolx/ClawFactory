// Package registry 实现注册中心，管理智能体注册、发现和健康检查
package registry

import (
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
)

// Registry 注册中心接口
type Registry interface {
	Register(req model.RegisterRequest) (model.AgentInfo, error)
	Heartbeat(agentID string) error
	GetAgent(agentID string) (model.AgentInfo, error)
	ListAgents() ([]model.AgentInfo, error)
	Deregister(agentID string) error
	CheckAndMarkOffline(timeout time.Duration) ([]string, error)
}
