package metrics

import "github.com/prometheus/client_golang/prometheus"

// TestableTasksTotal exposes the unexported tasksTotal field for external test packages.
func (c *PrometheusCollector) TestableTasksTotal() *prometheus.CounterVec {
	return c.tasksTotal
}

// TestableHTTPRequestsTotal exposes the unexported httpRequestsTotal field for external test packages.
func (c *PrometheusCollector) TestableHTTPRequestsTotal() *prometheus.CounterVec {
	return c.httpRequestsTotal
}
