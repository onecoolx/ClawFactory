package registry

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// StoreRegistry 基于 StateStore 的注册中心实现
type StoreRegistry struct {
	store store.StateStore
	mu    sync.RWMutex
}

// NewStoreRegistry 创建注册中心
func NewStoreRegistry(s store.StateStore) *StoreRegistry {
	return &StoreRegistry{store: s}
}

func generateAgentID(name, version string) string {
	h := sha256.Sum256([]byte(name + ":" + version))
	return fmt.Sprintf("agent-%x", h[:8])
}

func (r *StoreRegistry) Register(req model.RegisterRequest) (model.AgentInfo, error) {
	if req.Name == "" {
		return model.AgentInfo{}, fmt.Errorf("name is required")
	}
	if len(req.Capabilities) == 0 {
		return model.AgentInfo{}, fmt.Errorf("capabilities is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	agentID := generateAgentID(req.Name, req.Version)

	// 检查是否已注册（幂等性）
	existing, err := r.store.GetAgent(agentID)
	if err == nil && existing.AgentID != "" {
		// 已存在，恢复为 online
		if existing.Status == "offline" {
			r.store.UpdateAgentStatus(agentID, "online", time.Now())
			existing.Status = "online"
		}
		return existing, nil
	}

	now := time.Now()
	agent := model.AgentInfo{
		AgentID:       agentID,
		Name:          req.Name,
		Capabilities:  req.Capabilities,
		Version:       req.Version,
		Status:        "online",
		LastHeartbeat: now,
		Roles:         []string{"developer_agent"}, // 默认角色
		RegisteredAt:  now,
	}
	if err := r.store.SaveAgent(agent); err != nil {
		return model.AgentInfo{}, fmt.Errorf("save agent: %w", err)
	}
	return agent, nil
}

func (r *StoreRegistry) Heartbeat(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	agent, err := r.store.GetAgent(agentID)
	if err != nil {
		return fmt.Errorf("agent %s not found", agentID)
	}
	if agent.Status == "deregistered" {
		return fmt.Errorf("agent %s is deregistered", agentID)
	}

	now := time.Now()
	status := "online"
	return r.store.UpdateAgentStatus(agentID, status, now)
}

func (r *StoreRegistry) GetAgent(agentID string) (model.AgentInfo, error) {
	return r.store.GetAgent(agentID)
}

func (r *StoreRegistry) ListAgents() ([]model.AgentInfo, error) {
	return r.store.ListAgents()
}

func (r *StoreRegistry) Deregister(agentID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.store.UpdateAgentStatus(agentID, "deregistered", time.Now())
}

// CheckAndMarkOffline 检查并标记超时智能体为 offline
func (r *StoreRegistry) CheckAndMarkOffline(timeout time.Duration) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	agents, err := r.store.ListAgents()
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-timeout)
	var marked []string
	for _, a := range agents {
		if a.Status == "online" && a.LastHeartbeat.Before(cutoff) {
			if err := r.store.UpdateAgentStatus(a.AgentID, "offline", a.LastHeartbeat); err != nil {
				continue
			}
			marked = append(marked, a.AgentID)
		}
	}
	return marked, nil
}
