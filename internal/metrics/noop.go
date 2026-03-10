package metrics

// NoopCollector is a no-op MetricsCollector for when metrics are disabled.
type NoopCollector struct{}

// NewNoopCollector creates a new NoopCollector.
func NewNoopCollector() *NoopCollector { return &NoopCollector{} }

func (c *NoopCollector) IncTaskTotal(status string)                                    {}
func (c *NoopCollector) ObserveSchedulingDuration(seconds float64)                     {}
func (c *NoopCollector) SetQueueDepth(depth float64)                                   {}
func (c *NoopCollector) SetAgentsOnline(count float64)                                 {}
func (c *NoopCollector) ObserveWorkflowDuration(seconds float64)                       {}
func (c *NoopCollector) IncHTTPRequestTotal(method, path string, statusCode int)       {}
func (c *NoopCollector) ObserveHTTPRequestDuration(method, path string, seconds float64) {}
