// Package policy implements the policy engine for scheduling policies, authorization rules, and tool whitelists.
package policy

import "github.com/clawfactory/clawfactory/internal/model"

// PolicyEngine is the policy engine interface.
type PolicyEngine interface {
	// Task routing policy
	CanExecuteTask(agentID string, taskCapabilities []string) bool

	// Authorization check
	Authorize(req model.AuthorizeRequest) model.AuthorizeResponse

	// Retry policy
	ShouldRetry(taskID string) (bool, error)
	GetMaxRetries() int

	// Tool whitelist
	IsToolAllowed(agentID string, toolName string) bool
	CheckRateLimit(agentID string, toolName string) bool
}
