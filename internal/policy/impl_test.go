package policy

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

func newTestPolicyEngine(t testing.TB) (*ConfigPolicyEngine, *store.SQLiteStore) {
	tmpDB, err := os.CreateTemp("", "clawfactory-policy-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpDB.Close()
	t.Cleanup(func() { os.Remove(tmpDB.Name()) })

	s, err := store.NewSQLiteStore(tmpDB.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	config := model.PolicyConfig{
		MaxRetries: 3,
		Roles: map[string]model.RoleDefinition{
			"developer_agent": {
				Permissions: []model.Permission{
					{Resource: "shared_memory:*", Actions: []string{"read", "write"}},
					{Resource: "task:*", Actions: []string{"read", "update"}},
				},
			},
			"readonly_agent": {
				Permissions: []model.Permission{
					{Resource: "shared_memory:*", Actions: []string{"read"}},
					{Resource: "task:*", Actions: []string{"read"}},
				},
			},
		},
		ToolWhitelist: map[string]model.ToolPolicy{
			"developer_agent": {AllowedTools: []string{"llm_api", "file_write", "file_read"}, RateLimit: 60},
			"readonly_agent":  {AllowedTools: []string{"llm_api", "file_read"}, RateLimit: 5},
		},
	}

	pe := NewConfigPolicyEngineFromConfig(config, s)
	return pe, s
}

func seedAgent(s *store.SQLiteStore, agentID string, roles []string, caps []string) {
	s.SaveAgent(model.AgentInfo{
		AgentID: agentID, Name: "test-" + agentID, Capabilities: caps,
		Version: "1.0", Status: "online", Roles: roles,
		LastHeartbeat: time.Now(), RegisteredAt: time.Now(),
	})
}

// Property 13: 重试策略正确性
// **Validates: Requirements 8.3, 8.4**
func TestProperty13_RetryPolicy(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pe, s := newTestPolicyEngine(t)

		// 创建 workflow
		s.SaveWorkflow(
			model.WorkflowInstance{InstanceID: "wf-r", DefinitionID: "def-1", Status: "running", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			model.WorkflowDefinition{ID: "def-1", Name: "test"},
		)

		retryCount := rapid.IntRange(0, 5).Draw(t, "retryCount")
		taskID := rapid.StringMatching(`^rt-[a-z0-9]{4}$`).Draw(t, "taskID")
		s.SaveTask(model.Task{
			TaskID: taskID, WorkflowID: "wf-r", NodeID: "n1", Type: "test",
			Capabilities: []string{"c"}, Input: map[string]string{}, Output: map[string]string{},
			Status: "failed", RetryCount: retryCount, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		})

		shouldRetry, err := pe.ShouldRetry(taskID)
		if err != nil {
			t.Fatal(err)
		}
		expected := retryCount < pe.GetMaxRetries()
		if shouldRetry != expected {
			t.Fatalf("retryCount=%d, maxRetries=%d: got shouldRetry=%v, want %v",
				retryCount, pe.GetMaxRetries(), shouldRetry, expected)
		}
	})
}

// Property 14: 权限验证与审计
// **Validates: Requirements 9.3, 9.4**
func TestProperty14_AuthorizationAndAudit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pe, s := newTestPolicyEngine(t)

		role := rapid.SampledFrom([]string{"developer_agent", "readonly_agent"}).Draw(t, "role")
		seedAgent(s, "agent-auth", []string{role}, []string{"cap1"})

		action := rapid.SampledFrom([]string{"read", "write", "update", "delete"}).Draw(t, "action")
		resource := rapid.SampledFrom([]string{"shared_memory:wf-1/data", "task:task-1", "unknown:res"}).Draw(t, "resource")

		resp := pe.Authorize(model.AuthorizeRequest{
			AgentID: "agent-auth", Action: action, Resource: resource,
		})

		// 验证：developer_agent 有 shared_memory:* read/write 和 task:* read/update
		// readonly_agent 有 shared_memory:* read 和 task:* read
		roleDef := pe.config.Roles[role]
		expectedAllowed := false
		for _, perm := range roleDef.Permissions {
			if matchResource(perm.Resource, resource) && containsAction(perm.Actions, action) {
				expectedAllowed = true
				break
			}
		}

		if resp.Allowed != expectedAllowed {
			t.Fatalf("role=%s action=%s resource=%s: got allowed=%v, want %v",
				role, action, resource, resp.Allowed, expectedAllowed)
		}
	})
}

// Property 15: 工具白名单与速率限制
// **Validates: Requirements 10.2, 10.5**
func TestProperty15_ToolWhitelistAndRateLimit(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		pe, s := newTestPolicyEngine(t)

		role := rapid.SampledFrom([]string{"developer_agent", "readonly_agent"}).Draw(t, "role")
		seedAgent(s, "agent-tool", []string{role}, []string{"cap1"})

		tool := rapid.SampledFrom([]string{"llm_api", "file_write", "file_read", "dangerous_tool"}).Draw(t, "tool")

		allowed := pe.IsToolAllowed("agent-tool", tool)

		// 验证白名单
		tp := pe.config.ToolWhitelist[role]
		expectedAllowed := false
		for _, t := range tp.AllowedTools {
			if t == tool {
				expectedAllowed = true
				break
			}
		}
		if allowed != expectedAllowed {
			t.Fatalf("role=%s tool=%s: got allowed=%v, want %v", role, tool, allowed, expectedAllowed)
		}
	})
}

// Unit test: rate limit enforcement
func TestRateLimitEnforcement(t *testing.T) {
	pe, s := newTestPolicyEngine(t)
	seedAgent(s, "agent-rl", []string{"readonly_agent"}, []string{"cap1"})

	// readonly_agent has rate limit of 5 per minute
	for i := 0; i < 5; i++ {
		if !pe.CheckRateLimit("agent-rl", "llm_api") {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}
	// 6th call should be rejected
	if pe.CheckRateLimit("agent-rl", "llm_api") {
		t.Fatal("6th call should be rate limited")
	}
}
