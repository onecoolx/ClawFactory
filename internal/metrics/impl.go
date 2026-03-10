package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusCollector implements MetricsCollector using Prometheus client library.
type PrometheusCollector struct {
	tasksTotal              *prometheus.CounterVec
	schedulingDuration      prometheus.Histogram
	queueDepth              prometheus.Gauge
	agentsOnline            prometheus.Gauge
	workflowDuration        prometheus.Histogram
	httpRequestsTotal       *prometheus.CounterVec
	httpRequestDuration     *prometheus.HistogramVec
}

// NewPrometheusCollector creates a PrometheusCollector registered with the default registry.
func NewPrometheusCollector() *PrometheusCollector {
	return newCollector(prometheus.DefaultRegisterer)
}

// NewPrometheusCollectorForTest creates a PrometheusCollector registered with a custom registry.
// This allows tests to use isolated registries and avoid duplicate registration panics.
func NewPrometheusCollectorForTest(reg prometheus.Registerer) *PrometheusCollector {
	return newCollector(reg)
}

func newCollector(reg prometheus.Registerer) *PrometheusCollector {
	tasksTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "clawfactory_tasks_total",
		Help: "Total number of tasks by status.",
	}, []string{"status"})

	schedulingDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "clawfactory_scheduling_duration_seconds",
		Help: "Histogram of task scheduling duration in seconds.",
	})

	queueDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "clawfactory_queue_depth",
		Help: "Current number of pending tasks in the queue.",
	})

	agentsOnline := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "clawfactory_agents_online",
		Help: "Current number of online agents.",
	})

	workflowDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: "clawfactory_workflow_duration_seconds",
		Help: "Histogram of workflow execution duration in seconds.",
	})

	httpRequestsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "clawfactory_http_requests_total",
		Help: "Total number of HTTP requests by method, path, and status code.",
	}, []string{"method", "path", "status_code"})

	httpRequestDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "clawfactory_http_request_duration_seconds",
		Help: "Histogram of HTTP request duration in seconds.",
	}, []string{"method", "path"})

	reg.MustRegister(
		tasksTotal,
		schedulingDuration,
		queueDepth,
		agentsOnline,
		workflowDuration,
		httpRequestsTotal,
		httpRequestDuration,
	)

	return &PrometheusCollector{
		tasksTotal:          tasksTotal,
		schedulingDuration:  schedulingDuration,
		queueDepth:          queueDepth,
		agentsOnline:        agentsOnline,
		workflowDuration:    workflowDuration,
		httpRequestsTotal:   httpRequestsTotal,
		httpRequestDuration: httpRequestDuration,
	}
}

func (c *PrometheusCollector) IncTaskTotal(status string) {
	c.tasksTotal.WithLabelValues(status).Inc()
}

func (c *PrometheusCollector) ObserveSchedulingDuration(seconds float64) {
	c.schedulingDuration.Observe(seconds)
}

func (c *PrometheusCollector) SetQueueDepth(depth float64) {
	c.queueDepth.Set(depth)
}

func (c *PrometheusCollector) SetAgentsOnline(count float64) {
	c.agentsOnline.Set(count)
}

func (c *PrometheusCollector) ObserveWorkflowDuration(seconds float64) {
	c.workflowDuration.Observe(seconds)
}

func (c *PrometheusCollector) IncHTTPRequestTotal(method, path string, statusCode int) {
	c.httpRequestsTotal.WithLabelValues(method, path, strconv.Itoa(statusCode)).Inc()
}

func (c *PrometheusCollector) ObserveHTTPRequestDuration(method, path string, seconds float64) {
	c.httpRequestDuration.WithLabelValues(method, path).Observe(seconds)
}
