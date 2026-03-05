package registry

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

func newTestRegistry(t testing.TB) (*StoreRegistry, *store.SQLiteStore) {
	tmpDB, err := os.CreateTemp("", "clawfactory-reg-*.db")
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
	return NewStoreRegistry(s), s
}

// Property 1: 注册幂等性
// **Validates: Requirements 1.1, 1.2**
func TestProperty1_RegisterIdempotency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		reg, _ := newTestRegistry(t)
		name := rapid.StringMatching(`^agent-[a-z]{3}$`).Draw(t, "name")
		version := rapid.StringMatching(`^[0-9]\.[0-9]$`).Draw(t, "version")
		req := model.RegisterRequest{Name: name, Capabilities: []string{"cap1"}, Version: version}

		a1, err := reg.Register(req)
		if err != nil {
			t.Fatal(err)
		}
		a2, err := reg.Register(req)
		if err != nil {
			t.Fatal(err)
		}
		if a1.AgentID != a2.AgentID {
			t.Fatalf("idempotency violated: %s != %s", a1.AgentID, a2.AgentID)
		}
		agents, _ := reg.ListAgents()
		count := 0
		for _, a := range agents {
			if a.AgentID == a1.AgentID {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected 1 record, got %d", count)
		}
	})
}

// Property 2: 无效注册请求被拒绝
// **Validates: Requirements 1.3**
func TestProperty2_InvalidRegistrationRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		reg, _ := newTestRegistry(t)
		// 随机选择缺少 name 或 capabilities
		missingName := rapid.Bool().Draw(t, "missingName")
		var req model.RegisterRequest
		if missingName {
			req = model.RegisterRequest{Name: "", Capabilities: []string{"cap1"}, Version: "1.0"}
		} else {
			req = model.RegisterRequest{Name: "agent-x", Capabilities: []string{}, Version: "1.0"}
		}
		_, err := reg.Register(req)
		if err == nil {
			t.Fatal("expected error for invalid registration")
		}
	})
}

// Property 3: 心跳更新时间戳
// **Validates: Requirements 2.2**
func TestProperty3_HeartbeatUpdatesTimestamp(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		reg, _ := newTestRegistry(t)
		req := model.RegisterRequest{
			Name: rapid.StringMatching(`^hb-[a-z]{3}$`).Draw(t, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			t.Fatal(err)
		}
		before := time.Now()
		time.Sleep(time.Millisecond)
		if err := reg.Heartbeat(agent.AgentID); err != nil {
			t.Fatal(err)
		}
		updated, err := reg.GetAgent(agent.AgentID)
		if err != nil {
			t.Fatal(err)
		}
		if updated.LastHeartbeat.Before(before) {
			t.Fatalf("heartbeat timestamp not updated: %v < %v", updated.LastHeartbeat, before)
		}
	})
}

// Property 4: 心跳超时与恢复往返
// **Validates: Requirements 2.3, 2.4**
func TestProperty4_HeartbeatTimeoutAndRecovery(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		reg, s := newTestRegistry(t)
		req := model.RegisterRequest{
			Name: rapid.StringMatching(`^to-[a-z]{3}$`).Draw(t, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			t.Fatal(err)
		}

		// 模拟心跳超时：将 last_heartbeat 设为很久以前
		s.UpdateAgentStatus(agent.AgentID, "online", time.Now().Add(-10*time.Minute))

		marked, err := reg.CheckAndMarkOffline(90 * time.Second)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, id := range marked {
			if id == agent.AgentID {
				found = true
			}
		}
		if !found {
			t.Fatal("agent should be marked offline")
		}
		a, _ := reg.GetAgent(agent.AgentID)
		if a.Status != "offline" {
			t.Fatalf("status should be offline, got %s", a.Status)
		}

		// 恢复：重新心跳
		if err := reg.Heartbeat(agent.AgentID); err != nil {
			t.Fatal(err)
		}
		a, _ = reg.GetAgent(agent.AgentID)
		if a.Status != "online" {
			t.Fatalf("status should be online after heartbeat, got %s", a.Status)
		}
	})
}

// Property 10: 注销后不再分配任务
// **Validates: Requirements 6.3**
func TestProperty10_DeregisteredAgentNoTasks(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		reg, _ := newTestRegistry(t)
		req := model.RegisterRequest{
			Name: rapid.StringMatching(`^dr-[a-z]{3}$`).Draw(t, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			t.Fatal(err)
		}
		if err := reg.Deregister(agent.AgentID); err != nil {
			t.Fatal(err)
		}
		a, _ := reg.GetAgent(agent.AgentID)
		if a.Status != "deregistered" {
			t.Fatalf("status should be deregistered, got %s", a.Status)
		}
	})
}
