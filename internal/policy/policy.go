// Package policy 实现策略引擎，管理调度策略、授权规则和工具白名单
package policy

import "github.com/clawfactory/clawfactory/internal/model"

// PolicyEngine 策略引擎接口
type PolicyEngine interface {
	// 任务路由策略
	CanExecuteTask(agentID string, taskCapabilities []string) bool

	// 授权检查
	Authorize(req model.AuthorizeRequest) model.AuthorizeResponse

	// 重试策略
	ShouldRetry(taskID string) (bool, error)
	GetMaxRetries() int

	// 工具白名单
	IsToolAllowed(agentID string, toolName string) bool
	CheckRateLimit(agentID string, toolName string) bool
}
