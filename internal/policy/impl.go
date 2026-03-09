package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
)

// ConfigPolicyEngine is the configuration-file-based policy engine implementation.
type ConfigPolicyEngine struct {
	config model.PolicyConfig
	store  store.StateStore
	mu     sync.RWMutex
	// Rate limits: agent_id -> tool_name -> []time.Time (call timestamps)
	rateLimits map[string]map[string][]time.Time
}

// NewConfigPolicyEngine creates a policy engine from a JSON config file.
func NewConfigPolicyEngine(configPath string, s store.StateStore) (*ConfigPolicyEngine, error) {
	data, err := os.ReadFile(filepath.Clean(configPath))
	if err != nil {
		return nil, fmt.Errorf("read policy config: %w", err)
	}
	var config model.PolicyConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse policy config: %w", err)
	}
	return &ConfigPolicyEngine{
		config:     config,
		store:      s,
		rateLimits: make(map[string]map[string][]time.Time),
	}, nil
}

// NewConfigPolicyEngineFromConfig creates a policy engine from a parsed config.
func NewConfigPolicyEngineFromConfig(config model.PolicyConfig, s store.StateStore) *ConfigPolicyEngine {
	return &ConfigPolicyEngine{
		config:     config,
		store:      s,
		rateLimits: make(map[string]map[string][]time.Time),
	}
}

func (p *ConfigPolicyEngine) CanExecuteTask(agentID string, taskCapabilities []string) bool {
	agent, err := p.store.GetAgent(agentID)
	if err != nil {
		return false
	}
	capSet := make(map[string]bool)
	for _, c := range agent.Capabilities {
		capSet[c] = true
	}
	for _, c := range taskCapabilities {
		if capSet[c] {
			return true
		}
	}
	return false
}

func (p *ConfigPolicyEngine) Authorize(req model.AuthorizeRequest) model.AuthorizeResponse {
	agent, err := p.store.GetAgent(req.AgentID)
	if err != nil {
		resp := model.AuthorizeResponse{Allowed: false, Reason: "agent not found"}
		p.logAudit(req, resp)
		return resp
	}

	// Check agent role permissions
	for _, role := range agent.Roles {
		roleDef, ok := p.config.Roles[role]
		if !ok {
			continue
		}
		for _, perm := range roleDef.Permissions {
			if matchResource(perm.Resource, req.Resource) && containsAction(perm.Actions, req.Action) {
				resp := model.AuthorizeResponse{Allowed: true}
				p.logAudit(req, resp)
				return resp
			}
		}
	}

	resp := model.AuthorizeResponse{Allowed: false, Reason: "insufficient permissions"}
	p.logAudit(req, resp)
	return resp
}

func matchResource(pattern, resource string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, ":*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(resource, prefix) || strings.HasPrefix(resource+"/", prefix)
	}
	return pattern == resource
}

func containsAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

func (p *ConfigPolicyEngine) ShouldRetry(taskID string) (bool, error) {
	task, err := p.store.GetTask(taskID)
	if err != nil {
		return false, err
	}
	return task.RetryCount < p.config.MaxRetries, nil
}

func (p *ConfigPolicyEngine) GetMaxRetries() int {
	return p.config.MaxRetries
}

func (p *ConfigPolicyEngine) IsToolAllowed(agentID string, toolName string) bool {
	agent, err := p.store.GetAgent(agentID)
	if err != nil {
		return false
	}
	for _, role := range agent.Roles {
		tp, ok := p.config.ToolWhitelist[role]
		if !ok {
			continue
		}
		for _, t := range tp.AllowedTools {
			if t == toolName {
				return true
			}
		}
	}
	// If not in whitelist, log audit entry
	p.store.SaveAuditLog(model.AuditLogEntry{
		Timestamp: time.Now(), AgentID: agentID, Action: "call_tool",
		Resource: toolName, Allowed: false, Reason: "tool not in whitelist",
	})
	return false
}

func (p *ConfigPolicyEngine) CheckRateLimit(agentID string, toolName string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Minute)

	if _, ok := p.rateLimits[agentID]; !ok {
		p.rateLimits[agentID] = make(map[string][]time.Time)
	}

	// Clean expired records
	calls := p.rateLimits[agentID][toolName]
	var valid []time.Time
	for _, t := range calls {
		if t.After(windowStart) {
			valid = append(valid, t)
		}
	}

	// Get rate limit
	agent, err := p.store.GetAgent(agentID)
	if err != nil {
		return false
	}
	limit := 0
	for _, role := range agent.Roles {
		tp, ok := p.config.ToolWhitelist[role]
		if ok && tp.RateLimit > limit {
			limit = tp.RateLimit
		}
	}
	if limit == 0 {
		limit = 60 // default
	}

	if len(valid) >= limit {
		p.store.SaveAuditLog(model.AuditLogEntry{
			Timestamp: now, AgentID: agentID, Action: "call_tool",
			Resource: toolName, Allowed: false, Reason: "rate limit exceeded",
		})
		return false
	}

	valid = append(valid, now)
	p.rateLimits[agentID][toolName] = valid
	return true
}

func (p *ConfigPolicyEngine) logAudit(req model.AuthorizeRequest, resp model.AuthorizeResponse) {
	p.store.SaveAuditLog(model.AuditLogEntry{
		Timestamp: time.Now(), AgentID: req.AgentID, Action: req.Action,
		Resource: req.Resource, Allowed: resp.Allowed, Reason: resp.Reason,
	})
}
