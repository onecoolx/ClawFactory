package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/clawfactory/clawfactory/internal/model"
	"github.com/go-chi/chi/v5"
)

// createWebhookRequest is the request body for creating a webhook subscription.
type createWebhookRequest struct {
	URL        string   `json:"url"`
	EventTypes []string `json:"event_types"`
}

// createWebhookHandler handles POST /v1/admin/webhooks.
func (s *Server) createWebhookHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	var req createWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid JSON body")
		return
	}

	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "url is required")
		return
	}

	wh := model.WebhookSubscription{
		WebhookID:  generateUUID(),
		URL:        req.URL,
		EventTypes: req.EventTypes,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.Store.SaveWebhook(wh); err != nil {
		logger.Error("failed to save webhook", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create webhook")
		return
	}

	logger.Info("webhook created", "webhook_id", wh.WebhookID, "url", wh.URL)
	writeJSON(w, http.StatusCreated, wh)
}

// listWebhooksHandler handles GET /v1/admin/webhooks.
func (s *Server) listWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	webhooks, err := s.Store.ListWebhooks()
	if err != nil {
		logger.Error("failed to list webhooks", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list webhooks")
		return
	}

	logger.Info("listed webhooks", "count", len(webhooks))
	writeJSON(w, http.StatusOK, webhooks)
}

// deleteWebhookHandler handles DELETE /v1/admin/webhooks/{webhookID}.
func (s *Server) deleteWebhookHandler(w http.ResponseWriter, r *http.Request) {
	traceID := TraceIDFromContext(r.Context())
	logger := slog.With("trace_id", traceID, "component", "api")

	webhookID := chi.URLParam(r, "webhookID")
	if webhookID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "webhookID is required")
		return
	}

	if err := s.Store.DeleteWebhook(webhookID); err != nil {
		logger.Error("failed to delete webhook", "webhook_id", webhookID, "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete webhook")
		return
	}

	logger.Info("webhook deleted", "webhook_id", webhookID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
