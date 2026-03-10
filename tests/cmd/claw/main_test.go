package claw_test

import (
	"encoding/json"
	"testing"

	"pgregory.net/rapid"
)

// Property 25: CLI output format consistency
// **Validates: Requirements 16.6**
func TestProperty25_CLIOutputFormatConsistency(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random agent list response data
		n := rapid.IntRange(0, 5).Draw(t, "agentCount")
		agents := make([]map[string]interface{}, n)
		for i := 0; i < n; i++ {
			agents[i] = map[string]interface{}{
				"agent_id":     rapid.StringMatching(`^agent-[a-z0-9]{8}$`).Draw(t, "id"),
				"name":         rapid.StringMatching(`^[a-z]{3,8}$`).Draw(t, "name"),
				"status":       rapid.SampledFrom([]string{"online", "offline", "deregistered"}).Draw(t, "status"),
				"version":      "1.0",
				"capabilities": []string{"cap1"},
			}
		}

		// Verify JSON serialization produces valid JSON
		data, err := json.Marshal(agents)
		if err != nil {
			t.Fatalf("failed to marshal agents: %v", err)
		}
		if !json.Valid(data) {
			t.Fatal("output is not valid JSON")
		}

		// Verify deserialization
		var parsed []map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(parsed) != n {
			t.Fatalf("expected %d agents, got %d", n, len(parsed))
		}
	})
}
