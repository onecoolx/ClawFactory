package metrics_test

import (
	"strconv"
	"testing"

	"github.com/clawfactory/clawfactory/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"pgregory.net/rapid"
)

// gatherCounterValue extracts a counter value from a registry by metric name and label values.
func gatherCounterValue(t testing.TB, reg *prometheus.Registry, metricName string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != metricName {
			continue
		}
		for _, m := range mf.GetMetric() {
			if matchLabels(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// matchLabels checks if a metric's label pairs match the expected labels.
func matchLabels(pairs []*dto.LabelPair, expected map[string]string) bool {
	if len(pairs) != len(expected) {
		return false
	}
	for _, p := range pairs {
		v, ok := expected[p.GetName()]
		if !ok || v != p.GetValue() {
			return false
		}
	}
	return true
}

// Feature: v03-observability, Property 34: Task counter monotonically increasing
// **Validates: Requirements 1.2**
func TestProperty34_TaskCounterMonotonicallyIncreasing(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := prometheus.NewRegistry()
		mc := metrics.NewPrometheusCollectorForTest(reg)

		status := rapid.SampledFrom([]string{"completed", "failed", "pending", "assigned"}).Draw(rt, "status")
		n := rapid.IntRange(1, 50).Draw(rt, "count")

		for i := 0; i < n; i++ {
			mc.IncTaskTotal(status)
		}

		got := gatherCounterValue(t, reg, "clawfactory_tasks_total", map[string]string{"status": status})
		if got != float64(n) {
			rt.Fatalf("expected counter=%d for status=%q, got=%f", n, status, got)
		}
	})
}

// Feature: v03-observability, Property 35: HTTP request counter correctness
// **Validates: Requirements 1.7**
func TestProperty35_HTTPRequestCounterCorrectness(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		reg := prometheus.NewRegistry()
		mc := metrics.NewPrometheusCollectorForTest(reg)

		method := rapid.SampledFrom([]string{"GET", "POST", "PUT", "DELETE"}).Draw(rt, "method")
		path := rapid.SampledFrom([]string{"/v1/register", "/v1/heartbeat", "/v1/tasks", "/v1/authorize"}).Draw(rt, "path")
		statusCode := rapid.SampledFrom([]int{200, 201, 400, 401, 404, 500}).Draw(rt, "statusCode")
		n := rapid.IntRange(1, 50).Draw(rt, "count")

		for i := 0; i < n; i++ {
			mc.IncHTTPRequestTotal(method, path, statusCode)
		}

		got := gatherCounterValue(t, reg, "clawfactory_http_requests_total", map[string]string{
			"method":      method,
			"path":        path,
			"status_code": strconv.Itoa(statusCode),
		})
		if got != float64(n) {
			rt.Fatalf("expected counter=%d for method=%s path=%s status=%d, got=%f", n, method, path, statusCode, got)
		}
	})
}
