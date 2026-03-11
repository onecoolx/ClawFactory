package main_test

import (
	"strings"
	"testing"

	claw "github.com/clawfactory/clawfactory/cmd/claw"
	"pgregory.net/rapid"
)

// Feature: v031-reliability-cli, Property 45: Colorize function correctness
// Validates: Requirements 6.1, 6.2, 6.3, 6.6
func TestProperty45_ColorizeFunctionCorrectness(t *testing.T) {
	knownStatuses := []string{
		"online", "completed", "offline", "failed",
		"deregistered", "running", "assigned", "pending",
	}

	rapid.Check(t, func(t *rapid.T) {
		// Decide whether to test a known or unknown status
		useKnown := rapid.Bool().Draw(t, "useKnown")

		var status string
		if useKnown {
			status = rapid.SampledFrom(knownStatuses).Draw(t, "knownStatus")
		} else {
			// Generate a random string that is NOT in the known set
			status = rapid.StringMatching(`^[a-zA-Z_]{1,20}$`).Draw(t, "randomStatus")
		}

		result := claw.Colorize(status)

		_, isKnown := claw.StatusColors[status]
		if isKnown {
			// Known status: must be wrapped with ANSI color code + reset
			expectedColor := claw.StatusColors[status]
			expected := expectedColor + status + claw.ColorReset
			if result != expected {
				t.Fatalf("known status %q: expected %q, got %q", status, expected, result)
			}
			// The result must contain the original status string (readability)
			if !strings.Contains(result, status) {
				t.Fatalf("colorized output for %q does not contain the original status", status)
			}
		} else {
			// Unknown status: must return unchanged (no ANSI codes)
			if result != status {
				t.Fatalf("unknown status %q: expected unchanged %q, got %q", status, status, result)
			}
			// Must not contain ANSI escape sequences
			if strings.Contains(result, "\033[") {
				t.Fatalf("unknown status %q should not contain ANSI codes, got %q", status, result)
			}
		}
	})
}
