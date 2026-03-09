package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates and configures the HTTP router.
func NewRouter(srv *Server, validTokens []string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", healthHandler)

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
		})
	})

	return r
}
