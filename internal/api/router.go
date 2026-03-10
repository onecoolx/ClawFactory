package api

import (
	"github.com/clawfactory/clawfactory/internal/metrics"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewRouter creates and configures the HTTP router.
// mc is the MetricsCollector used by the metrics middleware.
// metricsEnabled controls whether the /metrics endpoint is registered.
func NewRouter(srv *Server, validTokens []string, mc metrics.MetricsCollector, metricsEnabled bool) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(TraceIDMiddleware)
	r.Use(MetricsMiddleware(mc))

	r.Get("/health", healthHandler)

	// Register /metrics endpoint outside auth middleware (no authentication required)
	if metricsEnabled {
		r.Handle("/metrics", promhttp.Handler())
	}

	r.Route("/v1", func(r chi.Router) {
		r.Use(TokenAuthMiddleware(validTokens))

		r.Post("/register", srv.registerHandler)
		r.Post("/heartbeat", srv.heartbeatHandler)
		r.Get("/tasks", srv.pullTaskHandler)
		r.Post("/tasks/{taskID}/status", srv.updateTaskStatusHandler)
		r.Post("/authorize", srv.authorizeHandler)
		r.Post("/log", srv.logHandler)

		r.Route("/admin", func(r chi.Router) {
			r.Get("/agents", srv.listAgentsHandler)
			r.Delete("/agents/{agentID}", srv.deregisterAgentHandler)
			r.Post("/workflows", srv.submitWorkflowHandler)
			r.Get("/workflows/{workflowID}", srv.getWorkflowStatusHandler)
			r.Get("/workflows/{workflowID}/artifacts", srv.getWorkflowArtifactsHandler)
			r.Get("/agents/{agentID}/logs", srv.getAgentLogsHandler)

			// Event and webhook management endpoints (v0.3)
			r.Get("/events", srv.listEventsHandler)
			r.Post("/webhooks", srv.createWebhookHandler)
			r.Get("/webhooks", srv.listWebhooksHandler)
			r.Delete("/webhooks/{webhookID}", srv.deleteWebhookHandler)
		})
	})

	return r
}
