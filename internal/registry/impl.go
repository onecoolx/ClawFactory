package registry

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// StoreRegistry is the StateStore-based registry implementation.
type StoreRegistry struct {
	store store.StateStore
	mu    sync.RWMutex
}

// NewStoreRegistry creates a new registry.
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

	// Check if already registered (idempotency)
	existing, err := r.store.GetAgent(agentID)
	if err == nil && existing.AgentID != "" {
		// Already exists, restore to online
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
		Roles:         []string{"developer_agent"}, // default role
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

// CheckAndMarkOffline checks and marks timed-out agents as offline.
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
