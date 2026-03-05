package registry

import (
	"os"
	"testing"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/clawfactory/clawfactory/internal/store"
	"pgregory.net/rapid"
)

func newTestRegistry(t *testing.T) (*StoreRegistry, *store.SQLiteStore) {
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

// Property 1: Registration idempotency
// **Validates: Requirements 1.1, 1.2**
func TestProperty1_RegisterIdempotency(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		name := "agent-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name")
		version := rapid.StringMatching("[0-9]\\.[0-9]").Draw(rt, "version")
		req := model.RegisterRequest{Name: name, Capabilities: []string{"cap1"}, Version: version}

		a1, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		a2, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		if a1.AgentID != a2.AgentID {
			rt.Fatalf("idempotency violated: %s != %s", a1.AgentID, a2.AgentID)
		}
		agents, _ := reg.ListAgents()
		count := 0
		for _, a := range agents {
			if a.AgentID == a1.AgentID {
				count++
			}
		}
		if count != 1 {
			rt.Fatalf("expected 1 record, got %d", count)
		}
	})
}

// Property 2: Invalid registration requests are rejected
// **Validates: Requirements 1.3**
func TestProperty2_InvalidRegistrationRejected(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		missingName := rapid.Bool().Draw(rt, "missingName")
		var req model.RegisterRequest
		if missingName {
			req = model.RegisterRequest{Name: "", Capabilities: []string{"cap1"}, Version: "1.0"}
		} else {
			req = model.RegisterRequest{Name: "agent-x", Capabilities: []string{}, Version: "1.0"}
		}
		_, err := reg.Register(req)
		if err == nil {
			rt.Fatal("expected error for invalid registration")
		}
	})
}

// Property 3: Heartbeat updates timestamp
// **Validates: Requirements 2.2**
func TestProperty3_HeartbeatUpdatesTimestamp(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		req := model.RegisterRequest{
			Name:         "hb-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		before := time.Now()
		time.Sleep(time.Millisecond)
		if err := reg.Heartbeat(agent.AgentID); err != nil {
			rt.Fatal(err)
		}
		updated, err := reg.GetAgent(agent.AgentID)
		if err != nil {
			rt.Fatal(err)
		}
		if updated.LastHeartbeat.Before(before) {
			rt.Fatalf("heartbeat timestamp not updated: %v < %v", updated.LastHeartbeat, before)
		}
	})
}

// Property 4: Heartbeat timeout and recovery round-trip
// **Validates: Requirements 2.3, 2.4**
func TestProperty4_HeartbeatTimeoutAndRecovery(t *testing.T) {
	reg, s := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		req := model.RegisterRequest{
			Name:         "to-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}

		// Simulate heartbeat timeout: set last_heartbeat to long ago
		s.UpdateAgentStatus(agent.AgentID, "online", time.Now().Add(-10*time.Minute))

		marked, err := reg.CheckAndMarkOffline(90 * time.Second)
		if err != nil {
			rt.Fatal(err)
		}
		found := false
		for _, id := range marked {
			if id == agent.AgentID {
				found = true
			}
		}
		if !found {
			rt.Fatal("agent should be marked offline")
		}
		a, _ := reg.GetAgent(agent.AgentID)
		if a.Status != "offline" {
			rt.Fatalf("status should be offline, got %s", a.Status)
		}

		// Recovery: heartbeat again
		if err := reg.Heartbeat(agent.AgentID); err != nil {
			rt.Fatal(err)
		}
		a, _ = reg.GetAgent(agent.AgentID)
		if a.Status != "online" {
			rt.Fatalf("status should be online after heartbeat, got %s", a.Status)
		}
	})
}

// Property 10: Deregistered agent no longer receives tasks
// **Validates: Requirements 6.3**
func TestProperty10_DeregisteredAgentNoTasks(t *testing.T) {
	reg, _ := newTestRegistry(t)

	rapid.Check(t, func(rt *rapid.T) {
		req := model.RegisterRequest{
			Name:         "dr-" + rapid.StringMatching("[a-z]{3}").Draw(rt, "name"),
			Capabilities: []string{"cap1"}, Version: "1.0",
		}
		agent, err := reg.Register(req)
		if err != nil {
			rt.Fatal(err)
		}
		if err := reg.Deregister(agent.AgentID); err != nil {
			rt.Fatal(err)
		}
		a, _ := reg.GetAgent(agent.AgentID)
		if a.Status != "deregistered" {
			rt.Fatalf("status should be deregistered, got %s", a.Status)
		}
	})
}
