package metrics_test

import (
	"testing"

	"github.com/clawfactory/clawfactory/internal/metrics"
)

// TestNoopCollectorNoPanic verifies that all NoopCollector methods can be called without panic.
func TestNoopCollectorNoPanic(t *testing.T) {
	nc := metrics.NewNoopCollector()

	nc.IncTaskTotal("completed")
	nc.IncTaskTotal("failed")
	nc.ObserveSchedulingDuration(1.5)
	nc.SetQueueDepth(10)
	nc.SetAgentsOnline(5)
	nc.ObserveWorkflowDuration(30.0)
	nc.IncHTTPRequestTotal("GET", "/v1/tasks", 200)
	nc.ObserveHTTPRequestDuration("POST", "/v1/register", 0.05)
}
