// Package registry implements the agent registry for registration, discovery, and health checks.
package registry

import (
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
)

// Registry is the agent registry interface.
type Registry interface {
	Register(req model.RegisterRequest) (model.AgentInfo, error)
	Heartbeat(agentID string) error
	GetAgent(agentID string) (model.AgentInfo, error)
	ListAgents() ([]model.AgentInfo, error)
	Deregister(agentID string) error
	CheckAndMarkOffline(timeout time.Duration) ([]string, error)
}
