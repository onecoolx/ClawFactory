// Package metrics defines the metrics collector interface for platform observability.
package metrics

// MetricsCollector defines the interface for recording platform metrics.
type MetricsCollector interface {
	// Task metrics
	IncTaskTotal(status string)
	ObserveSchedulingDuration(seconds float64)

	// Queue metrics
	SetQueueDepth(depth float64)

	// Agent metrics
	SetAgentsOnline(count float64)

	// Workflow metrics
	ObserveWorkflowDuration(seconds float64)

	// HTTP metrics
	IncHTTPRequestTotal(method, path string, statusCode int)
	ObserveHTTPRequestDuration(method, path string, seconds float64)
}
